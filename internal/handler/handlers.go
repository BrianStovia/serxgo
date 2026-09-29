package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"searxgo/internal/aggregator"
	"searxgo/internal/archive"
	"searxgo/internal/bypass"
	"searxgo/internal/config"
	"searxgo/internal/currency"
	"searxgo/internal/dns"
	"searxgo/internal/engine"
	"searxgo/internal/instant"
	"searxgo/internal/media"
	"searxgo/internal/newshub"
	"searxgo/internal/ping"
	"searxgo/internal/proxy"
	"searxgo/internal/stats"
	"searxgo/internal/subdomains"
	"searxgo/internal/tech"
	"searxgo/internal/tiktoklive"
	"searxgo/internal/weather"
	"searxgo/web"
)

// CategoryInfo defines metadata for UI search categories
type CategoryInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// DefaultCategoryList provides standard categories across SearXGo
var DefaultCategoryList = []CategoryInfo{
	{ID: "general", Name: "General", Icon: "🌐"},
	{ID: "images", Name: "Images", Icon: "🖼️"},
	{ID: "videos", Name: "Videos", Icon: "🎬"},
	{ID: "news", Name: "News", Icon: "📰"},
	{ID: "it", Name: "IT & Code", Icon: "💻"},
	{ID: "science", Name: "Science", Icon: "🔬"},
	{ID: "social", Name: "Social", Icon: "💬"},
	{ID: "files", Name: "Files", Icon: "📁"},
	{ID: "music", Name: "Music", Icon: "🎵"},
	{ID: "maps", Name: "Maps", Icon: "🗺️"},
}

type TempUploadedImage struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Data        []byte    `json:"-"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CreatedAt   time.Time `json:"created_at"`
}

type Handler struct {
	cfg            *config.Config
	aggregator     *aggregator.Aggregator
	instantService *instant.InstantService
	suggestService *SuggestService
	imageProxy     *proxy.ImageProxy
	limiter        *RateLimiter
	templates      *template.Template
	imageStoreMu   sync.RWMutex
	imageStore     map[string]*TempUploadedImage
	archiveClient  *archive.ArchiveClient
	techInspector  *tech.Inspector
	subFinder      *subdomains.Finder
	dnsService     *dns.Service
	pingService    *ping.Service
	bypassService     *bypass.Service
	mediaService      *media.Service
	newsHubService    *newshub.Service
	weatherService    *weather.Service
	currencyService   *currency.Service
	tiktokLiveService *tiktoklive.Service
}

func NewHandler(cfg *config.Config, agg *aggregator.Aggregator) (*Handler, error) {
	tmplFuncs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"safe": func(s string) template.HTML { return template.HTML(s) },
		"isMediaURL": func(u string) bool {
			lower := strings.ToLower(u)
			return strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be") ||
				strings.Contains(lower, "vimeo.com") || strings.Contains(lower, "dailymotion.com") ||
				strings.Contains(lower, "itunes.apple.com") || strings.Contains(lower, "mzstatic.com") ||
				strings.Contains(lower, "soundcloud.com") || strings.Contains(lower, "stream") ||
				strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") ||
				strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".ogg") ||
				strings.HasSuffix(lower, ".m4a") || strings.HasSuffix(lower, ".aac") ||
				strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".flac")
		},
		"extractDomain": func(rawURL string) string {
			raw := strings.TrimPrefix(rawURL, "https://")
			raw = strings.TrimPrefix(raw, "http://")
			parts := strings.Split(raw, "/")
			domain := parts[0]
			domain = strings.TrimPrefix(domain, "www.")
			return domain
		},
		"faviconURL": func(resolver, rawURL string) string {
			if resolver == "off" || resolver == "none" {
				return ""
			}
			raw := strings.TrimPrefix(rawURL, "https://")
			raw = strings.TrimPrefix(raw, "http://")
			parts := strings.Split(raw, "/")
			domain := parts[0]
			domain = strings.TrimPrefix(domain, "www.")
			if domain == "" {
				return ""
			}
			switch strings.ToLower(resolver) {
			case "google":
				return fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=32", domain)
			case "duckduckgo", "ddg":
				return fmt.Sprintf("https://icons.duckduckgo.com/ip2/%s.ico", domain)
			case "yandex":
				return fmt.Sprintf("https://favicon.yandex.net/favicon/%s", domain)
			case "allesedv":
				return fmt.Sprintf("https://f1.allesedv.com/32/%s", domain)
			case "kagi", "":
				return fmt.Sprintf("https://assets.kagi.com/proxy/favicons?domain=%s", domain)
			default:
				return fmt.Sprintf("https://assets.kagi.com/proxy/favicons?domain=%s", domain)
			}
		},
		"formatLatency": func(ms int64) string {
			if ms <= 0 {
				return "-"
			}
			if ms < 1000 {
				return fmt.Sprintf("%dms", ms)
			}
			return fmt.Sprintf("%.2fs", float64(ms)/1000.0)
		},
		"statPingPercent": func(ms int64, maxTime float64) int {
			if ms <= 0 {
				return 15
			}
			maxMs := maxTime * 1000.0
			if maxMs <= 0 {
				maxMs = 3000.0
			}
			pct := int((float64(ms) / maxMs) * 100.0)
			if pct < 10 {
				pct = 10
			}
			if pct > 100 {
				pct = 100
			}
			return pct
		},
		"rateColor": func(rate float64) string {
			if rate >= 80.0 {
				return "#10b981"
			} else if rate >= 50.0 {
				return "#f59e0b"
			}
			return "#ef4444"
		},
		"rateBg": func(rate float64) string {
			if rate >= 80.0 {
				return "rgba(16, 185, 129, 0.12)"
			} else if rate >= 50.0 {
				return "rgba(245, 158, 11, 0.12)"
			}
			return "rgba(239, 68, 68, 0.12)"
		},
		"latencyColor": func(ms int64) string {
			if ms < 800 {
				return "var(--accent-emerald)"
			} else if ms < 1500 {
				return "var(--accent-amber)"
			}
			return "var(--accent-rose)"
		},
	}

	tmpl, err := template.New("").Funcs(tmplFuncs).ParseFS(web.TemplatesFS, "templates/*.html", "templates/*.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded templates: %w", err)
	}

	return &Handler{
		cfg:             cfg,
		aggregator:      agg,
		instantService:  instant.NewInstantService(),
		suggestService:  NewSuggestService(),
		imageProxy:      proxy.NewImageProxy(),
		limiter:         NewRateLimiter(cfg.LimiterRate, cfg.LimiterBurst, cfg.LimiterEnabled),
		templates:       tmpl,
		imageStore:      make(map[string]*TempUploadedImage),
		archiveClient:   archive.NewArchiveClient(),
		techInspector:   tech.NewInspector(),
		subFinder:       subdomains.NewFinder(),
		dnsService:      dns.NewService(),
		pingService:     ping.NewService(),
		bypassService:   bypass.NewService(),
		mediaService:    media.NewService(),
		newsHubService:    newshub.NewService(),
		weatherService:    weather.NewService(),
		currencyService:   currency.NewService(),
		tiktokLiveService: tiktoklive.NewService(),
	}, nil
}

func extractDOI(rawURL string) string {
	lower := strings.ToLower(rawURL)
	if idx := strings.Index(lower, "10."); idx != -1 {
		candidate := rawURL[idx:]
		candidate = strings.TrimRight(candidate, ".,;:/")
		if len(candidate) > 7 && strings.Contains(candidate, "/") {
			return candidate
		}
	}
	return ""
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Standard SearXNG / SearXGo Web Endpoints
	mux.HandleFunc("GET /{$}", h.ServeIndex)
	mux.HandleFunc("GET /search", h.ServeSearch)
	mux.HandleFunc("POST /search", h.ServeSearch)
	mux.HandleFunc("GET /split", h.ServeSplit)
	mux.HandleFunc("GET /preferences", h.ServeSettings)
	mux.HandleFunc("POST /preferences", h.ServeSettings)
	mux.HandleFunc("GET /settings", h.ServeSettings)
	mux.HandleFunc("POST /settings", h.ServeSettings)
	mux.HandleFunc("GET /stats", h.ServeStats)
	mux.HandleFunc("GET /stats/data", h.ServeAPIStats)
	mux.HandleFunc("GET /preferences/clear", h.ServeClearCookies)
	mux.HandleFunc("POST /preferences/clear", h.ServeClearCookies)

	// Feeds, OpenSearch & PWA
	mux.HandleFunc("GET /opensearch.xml", h.ServeOpenSearch)
	mux.HandleFunc("GET /manifest.webmanifest", h.ServeManifest)
	mux.HandleFunc("GET /sw.js", h.ServeServiceWorker)
	mux.HandleFunc("GET /robots.txt", h.ServeRobotsTxt)
	mux.HandleFunc("GET /favicon.ico", h.ServeFavicon)
	mux.HandleFunc("GET /engine_descriptions.json", h.ServeEngineDescriptions)

	// Autocomplete & Suggestions
	mux.HandleFunc("GET /suggest", h.ServeSuggest)
	mux.HandleFunc("GET /autocompleter", h.ServeAutocompleter)

	// Public REST API
	mux.HandleFunc("GET /api/search", h.ServeAPI)
	mux.HandleFunc("GET /api/suggest", h.ServeSuggest)
	mux.HandleFunc("GET /api/stats", h.ServeAPIStats)
	mux.HandleFunc("GET /api/watchdog", h.ServeWatchdog)

	// Reverse Image Search & File Upload
	mux.HandleFunc("POST /upload/image", h.ServeUploadImage)
	mux.HandleFunc("GET /upload/image/{id}", h.ServeTempImage)
	mux.HandleFunc("GET /api/reverse-image", h.ServeReverseImage)
	mux.HandleFunc("POST /api/reverse-image", h.ServeReverseImage)

	// Autonomous Watchdog RSS Feed
	mux.HandleFunc("GET /watchdog", h.ServeWatchdog)

	// OSINT, Security & Analysis Tools
	mux.HandleFunc("GET /dorks", h.ServeDorks)
	mux.HandleFunc("GET /api/dorks", h.ServeAPIDorks)

	mux.HandleFunc("GET /scrub", h.ServeScrub)
	mux.HandleFunc("POST /api/scrub/inspect", h.ServeAPIScrubInspect)
	mux.HandleFunc("POST /api/scrub/clean", h.ServeAPIScrubClean)

	mux.HandleFunc("GET /recon", h.ServeRecon)
	mux.HandleFunc("GET /api/recon", h.ServeAPIRecon)

	mux.HandleFunc("GET /reader", h.ServeReader)
	mux.HandleFunc("GET /api/reader", h.ServeAPIReader)

	mux.HandleFunc("GET /sherlock", h.ServeSherlock)
	mux.HandleFunc("GET /api/sherlock", h.ServeAPISherlock)
	mux.HandleFunc("POST /api/sherlock", h.ServeAPISherlock)

	mux.HandleFunc("GET /threat", h.ServeThreat)
	mux.HandleFunc("GET /api/threat", h.ServeAPIThreat)
	mux.HandleFunc("POST /api/threat", h.ServeAPIThreat)

	mux.HandleFunc("GET /tech", h.ServeTech)
	mux.HandleFunc("GET /api/tech", h.ServeAPITech)

	mux.HandleFunc("GET /subdomains", h.ServeSubdomains)
	mux.HandleFunc("GET /api/subdomains", h.ServeAPISubdomains)

	mux.HandleFunc("GET /dns", h.ServeDNS)
	mux.HandleFunc("GET /api/dns", h.ServeAPIDNS)

	mux.HandleFunc("GET /ping", h.ServePing)
	mux.HandleFunc("GET /api/ping", h.ServeAPIPing)

	mux.HandleFunc("GET /bypass", h.ServeBypass)
	mux.HandleFunc("GET /api/bypass", h.ServeAPIBypass)

	mux.HandleFunc("GET /media", h.ServeMedia)
	mux.HandleFunc("GET /api/media", h.ServeAPIMedia)

	// TikTok Live Direct Stream Viewer & Proxy
	mux.HandleFunc("GET /tiktoklive", h.ServeTikTokLive)
	mux.HandleFunc("GET /live/tiktok", h.ServeTikTokLive)
	mux.HandleFunc("GET /api/tiktoklive/info", h.ServeAPITikTokLiveInfo)
	mux.HandleFunc("GET /api/tiktoklive/proxy", h.ServeAPITikTokLiveProxy)

	mux.HandleFunc("GET /qr", h.ServeQR)
	mux.HandleFunc("GET /api/qr/wifi", h.ServeAPIQRWiFi)

	mux.HandleFunc("GET /news-hub", h.ServeNewsHub)
	mux.HandleFunc("GET /api/newshub", h.ServeAPINewsHub)

	mux.HandleFunc("GET /weather", h.ServeWeather)
	mux.HandleFunc("GET /api/weather", h.ServeAPIWeather)

	mux.HandleFunc("GET /currency", h.ServeCurrency)
	mux.HandleFunc("GET /api/currency/convert", h.ServeAPICurrencyConvert)
	mux.HandleFunc("GET /api/currency/cryptos", h.ServeAPICurrencyCrypto)
	mux.HandleFunc("GET /api/currency/currencies", h.ServeAPICurrencyCurrencies)
	mux.HandleFunc("GET /api/currency/rates", h.ServeAPICurrencyRates)

	mux.HandleFunc("GET /api-tester", h.ServeAPITester)
	mux.HandleFunc("POST /api/api-tester/execute", h.ServeAPIExecuteRequest)

	mux.HandleFunc("GET /cloud-recon", h.ServeCloudRecon)
	mux.HandleFunc("GET /api/cloud-recon", h.ServeAPICloudRecon)

	mux.HandleFunc("GET /cve", h.ServeCVE)
	mux.HandleFunc("GET /api/cve", h.ServeAPICVE)

	mux.HandleFunc("GET /ip-intel", h.ServeIPIntel)
	mux.HandleFunc("GET /api/ip-intel", h.ServeAPIIPIntel)

	// Interactive Knowledge Graph & Client Goggles
	mux.HandleFunc("GET /graph", h.ServeGraph)
	mux.HandleFunc("GET /api/graph", h.ServeAPIGraph)
	mux.HandleFunc("POST /api/goggles/validate", h.ServeAPIGogglesValidate)

	// Time Machine / Archive.org
	mux.HandleFunc("GET /archive", h.ServeArchive)
	mux.HandleFunc("GET /api/archive", h.ServeAPIArchive)

	// Health and Telemetry
	mux.HandleFunc("GET /healthz", h.ServeHealthz)
	mux.HandleFunc("GET /metrics", h.ServeMetrics)
	mux.HandleFunc("GET /config", h.ServeConfig)

	// Image Proxy
	mux.HandleFunc("GET /proxy/image", h.imageProxy.ServeHTTP)
	mux.HandleFunc("GET /image_proxy", h.imageProxy.ServeHTTP) // SearXNG official alias

	// Static Assets with Cache-Control headers to prevent stale CSS/JS
	fileServer := http.FileServer(http.FS(web.StaticFS))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		fileServer.ServeHTTP(w, r)
	}))
}

func (h *Handler) ServeManifest(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(web.StaticFS, "static/manifest.webmanifest")
	if err != nil {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

func (h *Handler) ServeServiceWorker(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(web.StaticFS, "static/sw.js")
	if err != nil {
		http.Error(w, "Service Worker not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

func (h *Handler) ServeHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "OK",
		"version":      "1.0.0",
		"engine_count": len(engine.FullEngineCatalog),
		"uptime":       "running",
	})
}

func (h *Handler) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	metrics := stats.GlobalTracker.GetOpenMetrics()
	w.Write([]byte(metrics))
}

func (h *Handler) ServeConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"instance_name": "SearXGo",
		"version":       "1.0.0",
		"categories": []string{
			"general", "images", "videos", "news", "music", "it", "science", "files", "social", "maps",
		},
		"default_category": "general",
		"search_formats":   []string{"html", "json", "csv", "rss"},
		"autocomplete_backends": []string{
			"duckduckgo", "google", "brave", "bing", "wikipedia", "startpage", "qwant", "all",
		},
		"engine_count":    len(engine.FullEngineCatalog),
		"safe_search":     []string{"off", "moderate", "strict"},
		"themes":          []string{"dark", "light", "black", "oled", "dracula", "nord", "mocha", "macchiato", "cyberpunk"},
		"doi_resolvers":   []string{"oadoi.org", "sci-hub.se", "sci-hub.st", "sci-hub.ru", "libgen.is", "unpaywall.org"},
		"limiter":          false,
		"unlimited_search": true,
		"max_page":         0,
		"infinite_scroll":  true,
		"public_instance":  true,
		"image_proxy":      true,
		"uptime":           "running",
	})
}

func (h *Handler) ServeRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /search\nDisallow: /api/\nDisallow: /proxy/\nDisallow: /image_proxy\nDisallow: /autocompleter\nDisallow: /preferences\nDisallow: /settings\n")
}

func (h *Handler) ServeFavicon(w http.ResponseWriter, r *http.Request) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="45" fill="#6366f1"/><circle cx="50" cy="50" r="25" fill="#06b6d4"/></svg>`
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(svg))
}

// WrapMiddleware attaches rate limiting and global security headers
func (h *Handler) WrapMiddleware(next http.Handler) http.Handler {
	return SecurityHeadersMiddleware(h.limiter.Middleware(next))
}

// SecurityHeadersMiddleware sets modern HTTP security headers matching SearXNG
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net https://fonts.googleapis.com; img-src 'self' data: https: http:; font-src 'self' data: https://fonts.gstatic.com; connect-src 'self'; frame-src 'self' https://www.youtube.com https://www.dailymotion.com https://player.vimeo.com https://www.bilibili.com https://www.tiktok.com;")
		next.ServeHTTP(w, r)
	})
}
