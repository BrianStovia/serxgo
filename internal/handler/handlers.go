package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"searxgo/internal/aggregator"
	"searxgo/internal/archive"
	"searxgo/internal/bangs"
	"searxgo/internal/bypass"
	"searxgo/internal/config"
	"searxgo/internal/currency"
	"searxgo/internal/dns"
	"searxgo/internal/dorks"
	"searxgo/internal/engine"
	"searxgo/internal/instant"
	"searxgo/internal/media"
	"searxgo/internal/models"
	"searxgo/internal/newshub"
	"searxgo/internal/ping"
	"searxgo/internal/proxy"
	"searxgo/internal/qr"
	"searxgo/internal/reader"
	"searxgo/internal/recon"
	"searxgo/internal/scrub"
	"searxgo/internal/stats"
	"searxgo/internal/subdomains"
	"searxgo/internal/tech"
	"searxgo/internal/threat"
	"searxgo/internal/weather"
	"searxgo/web"
)

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
	bypassService  *bypass.Service
	mediaService   *media.Service
	newsHubService *newshub.Service
	weatherService *weather.Service
	currencyService *currency.Service
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
	}

	tmpl, err := template.New("").Funcs(tmplFuncs).ParseFS(web.TemplatesFS, "templates/*.html", "templates/*.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded templates: %w", err)
	}

	return &Handler{
		cfg:            cfg,
		aggregator:     agg,
		instantService: instant.NewInstantService(),
		suggestService: NewSuggestService(),
		imageProxy:     proxy.NewImageProxy(),
		limiter:        NewRateLimiter(cfg.LimiterRate, cfg.LimiterBurst, cfg.LimiterEnabled),
		templates:      tmpl,
		imageStore:     make(map[string]*TempUploadedImage),
		archiveClient:  archive.NewArchiveClient(),
		techInspector:  tech.NewInspector(),
		subFinder:      subdomains.NewFinder(),
		dnsService:     dns.NewService(),
		pingService:    ping.NewService(),
		bypassService:  bypass.NewService(),
		mediaService:   media.NewService(),
		newsHubService: newshub.NewService(),
		weatherService: weather.NewService(),
		currencyService: currency.NewService(),
	}, nil
}

// RegisterRoutes sets up all URL paths on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Web Pages
	mux.HandleFunc("GET /{$}", h.ServeIndex)
	mux.HandleFunc("GET /search", h.ServeSearch)
	mux.HandleFunc("POST /search", h.ServeSearch) // SearXNG POST support
	mux.HandleFunc("GET /settings", h.ServeSettings)
	mux.HandleFunc("POST /settings", h.ServeSettings)
	mux.HandleFunc("GET /preferences", h.ServeSettings)  // SearXNG official alias
	mux.HandleFunc("POST /preferences", h.ServeSettings) // SearXNG official alias
	mux.HandleFunc("GET /clear_cookies", h.ServeClearCookies)
	mux.HandleFunc("GET /stats", h.ServeStats)
	mux.HandleFunc("GET /stats/errors", h.ServeStats) // SearXNG official alias
	mux.HandleFunc("GET /opensearch.xml", h.ServeOpenSearch)
	mux.HandleFunc("GET /manifest.webmanifest", h.ServeManifest)
	mux.HandleFunc("GET /manifest.json", h.ServeManifest)
	mux.HandleFunc("GET /sw.js", h.ServeServiceWorker)
	mux.HandleFunc("GET /favicon.ico", h.ServeFavicon)
	mux.HandleFunc("GET /robots.txt", h.ServeRobotsTxt)
	mux.HandleFunc("GET /healthz", h.ServeHealthz)
	mux.HandleFunc("GET /health", h.ServeHealthz)
	mux.HandleFunc("GET /config", h.ServeConfig)
	mux.HandleFunc("GET /metrics", h.ServeMetrics)         // SearXNG OpenMetrics / Prometheus parity
	mux.HandleFunc("GET /openmetrics", h.ServeMetrics)     // SearXNG OpenMetrics / Prometheus parity

	// API Endpoints, Autocompleter & Descriptions
	mux.HandleFunc("GET /api/search", h.ServeAPI)
	mux.HandleFunc("POST /api/search", h.ServeAPI)
	mux.HandleFunc("GET /api/suggest", h.ServeSuggest)
	mux.HandleFunc("GET /autocompleter", h.ServeAutocompleter)
	mux.HandleFunc("GET /api/stats", h.ServeAPIStats)
	mux.HandleFunc("GET /status.json", h.ServeAPIStats)
	mux.HandleFunc("GET /status", h.ServeAPIStats)
	mux.HandleFunc("GET /engine_descriptions.json", h.ServeEngineDescriptions)
	mux.HandleFunc("GET /engines", h.ServeEngineDescriptions)
	mux.HandleFunc("GET /split", h.ServeSplit)
	mux.HandleFunc("GET /watchdog", h.ServeWatchdog)
	mux.HandleFunc("GET /api/watchdog", h.ServeWatchdog)

	// Visual Reverse Image Search Endpoints
	mux.HandleFunc("POST /api/reverse-image", h.ServeReverseImage)
	mux.HandleFunc("GET /api/reverse-image", h.ServeReverseImage)
	mux.HandleFunc("POST /upload/image", h.ServeUploadImage)
	mux.HandleFunc("GET /upload/image/{id}", h.ServeTempImage)

	// Google Dorking Recon Suite
	mux.HandleFunc("GET /dorks", h.ServeDorks)
	mux.HandleFunc("GET /api/dorks", h.ServeAPIDorks)

	// EXIF Metadata Stripper & Privacy Cleaner
	mux.HandleFunc("GET /scrub", h.ServeScrub)
	mux.HandleFunc("POST /api/scrub/inspect", h.ServeAPIScrubInspect)
	mux.HandleFunc("POST /api/scrub/clean", h.ServeAPIScrubClean)

	// Domain Recon & Security Auditor
	mux.HandleFunc("GET /recon", h.ServeRecon)
	mux.HandleFunc("GET /api/recon", h.ServeAPIRecon)

	// Clean Reader View
	mux.HandleFunc("GET /reader", h.ServeReader)
	mux.HandleFunc("GET /api/reader", h.ServeAPIReader)

	// OSINT Sherlock Workspace & Account Recon
	mux.HandleFunc("GET /sherlock", h.ServeSherlock)
	mux.HandleFunc("GET /osint", h.ServeSherlock)
	mux.HandleFunc("GET /api/sherlock", h.ServeAPISherlock)
	mux.HandleFunc("POST /api/sherlock", h.ServeAPISherlock)

	// URL Threat Intelligence Scanner & Sandbox
	mux.HandleFunc("GET /threat", h.ServeThreat)
	mux.HandleFunc("GET /scan", h.ServeThreat)
	mux.HandleFunc("GET /api/threat", h.ServeAPIThreat)
	mux.HandleFunc("POST /api/threat", h.ServeAPIThreat)

	// Interactive Knowledge & Entity Graph
	mux.HandleFunc("GET /graph", h.ServeGraph)
	mux.HandleFunc("GET /api/graph", h.ServeAPIGraph)

	// Internet Time Machine & Wayback Archive
	mux.HandleFunc("GET /archive", h.ServeArchive)
	mux.HandleFunc("GET /api/archive", h.ServeAPIArchive)

	// Tech Stack & Security Header Inspector
	mux.HandleFunc("GET /tech", h.ServeTech)
	mux.HandleFunc("GET /api/tech", h.ServeAPITech)

	// Subdomain Enumeration via CT Logs
	mux.HandleFunc("GET /subdomains", h.ServeSubdomains)
	mux.HandleFunc("GET /api/subdomains", h.ServeAPISubdomains)

	// Global DNS Propagation & Records
	mux.HandleFunc("GET /dns", h.ServeDNS)
	mux.HandleFunc("GET /api/dns", h.ServeAPIDNS)

	// Is It Down & Port Ping Tester
	mux.HandleFunc("GET /ping", h.ServePing)
	mux.HandleFunc("GET /uptime", h.ServePing)
	mux.HandleFunc("GET /api/ping", h.ServeAPIPing)

	// Paywall Bypass & Clean Mirror
	mux.HandleFunc("GET /bypass", h.ServeBypass)
	mux.HandleFunc("GET /api/bypass", h.ServeAPIBypass)

	// Ad-Free Media & Video Downloader
	mux.HandleFunc("GET /media", h.ServeMedia)
	mux.HandleFunc("GET /api/media", h.ServeAPIMedia)

	// QR Code & Wi-Fi Studio
	mux.HandleFunc("GET /qr", h.ServeQR)
	mux.HandleFunc("GET /api/qr/wifi", h.ServeAPIQRWiFi)

	// World News Pulse (Live Global News Portals)
	mux.HandleFunc("GET /news-hub", h.ServeNewsHub)
	mux.HandleFunc("GET /pulse", h.ServeNewsHub)
	mux.HandleFunc("GET /worldnews", h.ServeNewsHub)
	mux.HandleFunc("GET /api/news-hub", h.ServeAPINewsHub)

	// Live Weather Radar & Global Forecast
	mux.HandleFunc("GET /weather", h.ServeWeather)
	mux.HandleFunc("GET /api/weather", h.ServeAPIWeather)

	// Live Currency & Crypto Exchange Converter
	mux.HandleFunc("GET /currency", h.ServeCurrency)
	mux.HandleFunc("GET /api/currency/convert", h.ServeAPICurrencyConvert)
	mux.HandleFunc("GET /api/currency/crypto", h.ServeAPICurrencyCrypto)
	mux.HandleFunc("GET /api/currency/currencies", h.ServeAPICurrencyCurrencies)
	mux.HandleFunc("GET /api/currency/rates", h.ServeAPICurrencyRates)

	// Search Goggles - Domain Block/Boost API
	mux.HandleFunc("POST /api/goggles/validate", h.ServeAPIGogglesValidate)

	// Privacy Proxy
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

func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Engines": engine.DefaultRegistry.GetEngineInfos(),
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) parseSearchRequest(r *http.Request) models.SearchRequest {
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
	}

	getParam := func(key string) string {
		if val := r.URL.Query().Get(key); val != "" {
			return val
		}
		if r.PostForm != nil {
			return r.PostFormValue(key)
		}
		return ""
	}

	rawQ := strings.TrimSpace(getParam("q"))
	catStr := strings.ToLower(getParam("category"))
	page, _ := strconv.Atoi(getParam("page"))
	if page <= 0 {
		page, _ = strconv.Atoi(getParam("pageno"))
	}
	if page <= 0 {
		page, _ = strconv.Atoi(getParam("p"))
	}
	timeRange := strings.ToLower(getParam("time_range"))
	lang := strings.ToLower(getParam("language"))
	format := strings.ToLower(getParam("format"))

	if page <= 0 {
		page = 1
	}

	cat := models.CategoryGeneral
	if getParam("category_images") == "1" || getParam("category_images") == "on" {
		cat = models.CategoryImages
	} else if getParam("category_videos") == "1" || getParam("category_videos") == "on" {
		cat = models.CategoryVideos
	} else if getParam("category_news") == "1" || getParam("category_news") == "on" {
		cat = models.CategoryNews
	} else if getParam("category_it") == "1" || getParam("category_it") == "on" {
		cat = models.CategoryIT
	} else if getParam("category_science") == "1" || getParam("category_science") == "on" {
		cat = models.CategoryScience
	} else if getParam("category_files") == "1" || getParam("category_files") == "on" {
		cat = models.CategoryFiles
	} else if getParam("category_music") == "1" || getParam("category_music") == "on" {
		cat = models.CategoryMusic
	} else if getParam("category_social") == "1" || getParam("category_social") == "on" {
		cat = models.CategorySocial
	} else if getParam("category_maps") == "1" || getParam("category_maps") == "on" {
		cat = models.CategoryMaps
	} else {
		switch catStr {
		case "images", "image":
			cat = models.CategoryImages
		case "videos", "video":
			cat = models.CategoryVideos
		case "news":
			cat = models.CategoryNews
		case "it", "code":
			cat = models.CategoryIT
		case "science", "sci":
			cat = models.CategoryScience
		case "social":
			cat = models.CategorySocial
		case "files", "torrents":
			cat = models.CategoryFiles
		case "music", "audio":
			cat = models.CategoryMusic
		case "maps", "map":
			cat = models.CategoryMaps
		}
	}

	// SafeSearch level (Default to SafeSearchOff for 100% uncensored & unfiltered results)
	safeSearch := models.SafeSearchOff
	if cookie, err := r.Cookie("searxgo_safesearch"); err == nil {
		if s, err := strconv.Atoi(cookie.Value); err == nil {
			safeSearch = models.SafeSearchLevel(s)
		}
	}
	if s := getParam("safesearch"); s != "" {
		if val, err := strconv.Atoi(s); err == nil {
			safeSearch = models.SafeSearchLevel(val)
		}
	}

	// Language fallback to cookie
	if lang == "" {
		if cookie, err := r.Cookie("searxgo_language"); err == nil && cookie.Value != "" {
			lang = cookie.Value
		}
	}

	// Enabled engines from cookie
	var enabledEngines []string
	if cookie, err := r.Cookie("searxgo_engines"); err == nil && cookie.Value != "" {
		for _, eng := range strings.Split(cookie.Value, ",") {
			if trimmed := strings.TrimSpace(eng); trimmed != "" {
				enabledEngines = append(enabledEngines, trimmed)
			}
		}
	}

	// Privacy frontend redirects
	enableRedirects := true
	if cookie, err := r.Cookie("searxgo_redirects"); err == nil && cookie.Value == "false" {
		enableRedirects = false
	}

	// DOI Resolver preference
	doiResolver := "oadoi.org"
	if cookie, err := r.Cookie("searxgo_doi_resolver"); err == nil && cookie.Value != "" {
		doiResolver = cookie.Value
	}
	if d := getParam("doi_resolver"); d != "" {
		doiResolver = d
	}

	// Results on new tab preference
	openInNewTab := false
	if cookie, err := r.Cookie("searxgo_newtab"); err == nil && cookie.Value == "true" {
		openInNewTab = true
	} else if cookie, err := r.Cookie("searxgo_new_tab"); err == nil && cookie.Value == "true" {
		openInNewTab = true
	}
	if getParam("results_on_new_tab") == "1" || getParam("results_on_new_tab") == "true" {
		openInNewTab = true
	}

	// Tracker URL remover preference (SearXNG tracker_url_remover plugin - default true)
	removeTrackers := true
	if cookie, err := r.Cookie("searxgo_tracker_remover"); err == nil && cookie.Value == "false" {
		removeTrackers = false
	}
	if getParam("tracker_remover") == "0" || getParam("tracker_remover") == "false" {
		removeTrackers = false
	}

	// Autocomplete backend preference
	autocomplete := "all"
	if cookie, err := r.Cookie("searxgo_autocomplete"); err == nil && cookie.Value != "" {
		autocomplete = cookie.Value
	}
	if ac := getParam("autocomplete"); ac != "" {
		autocomplete = ac
	}

	// Page size preference
	pageSize := h.cfg.DefaultPageSize
	if cookie, err := r.Cookie("searxgo_page_size"); err == nil && cookie.Value != "" {
		if ps, err := strconv.Atoi(cookie.Value); err == nil && ps > 0 {
			pageSize = ps
		}
	}
	if psStr := getParam("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 {
			pageSize = ps
		}
	} else if countStr := getParam("count"); countStr != "" {
		if ps, err := strconv.Atoi(countStr); err == nil && ps > 0 {
			pageSize = ps
		}
	} else if limitStr := getParam("limit"); limitStr != "" {
		if ps, err := strconv.Atoi(limitStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	// Country and Region preferences
	country := strings.ToLower(getParam("country"))
	if country == "" {
		if cookie, err := r.Cookie("searxgo_country"); err == nil && cookie.Value != "" {
			country = strings.ToLower(cookie.Value)
		}
	}

	region := strings.ToLower(getParam("region"))
	if region == "" {
		if cookie, err := r.Cookie("searxgo_region"); err == nil && cookie.Value != "" {
			region = strings.ToLower(cookie.Value)
		}
	}

	deepSearch := false
	if getParam("deep") == "1" || getParam("deep") == "true" || getParam("deep_search") == "1" {
		deepSearch = true
	}

	crossCategory := false
	if getParam("cross") == "1" || getParam("cross_category") == "1" || getParam("federated") == "1" {
		crossCategory = true
	}

	return models.SearchRequest{
		Query:           rawQ,
		RawQuery:        rawQ,
		Category:        cat,
		Page:            page,
		PageSize:        pageSize,
		Country:         country,
		Region:          region,
		DeepSearch:      deepSearch,
		CrossCategory:   crossCategory,
		TimeRange:       timeRange,
		Language:        lang,
		SafeSearch:      safeSearch,
		EnabledEngines:  enabledEngines,
		Format:          format,
		EnableRedirects: enableRedirects,
		DOIResolver:     doiResolver,
		OpenInNewTab:    openInNewTab,
		RemoveTrackers:  removeTrackers,
		Autocomplete:    autocomplete,
	}
}

func (h *Handler) getClientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	if trueIP := r.Header.Get("True-Client-IP"); trueIP != "" {
		return strings.TrimSpace(trueIP)
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (h *Handler) ServeSearch(w http.ResponseWriter, r *http.Request) {
	req := h.parseSearchRequest(r)

	if req.Query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// 1. Check for Direct Bang redirect (e.g. !yt! lo fi or !gh! kubernetes)
	bangParsed := bangs.ParseBangs(req.Query, req.Category)
	if bangParsed.DirectRedirectURL != "" {
		http.Redirect(w, r, bangParsed.DirectRedirectURL, http.StatusFound)
		return
	}

	// 2. Format handlers (JSON, RSS, CSV)
	if req.Format == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.ServeAPI(w, r)
		return
	}
	if req.Format == "rss" || req.Format == "atom" {
		h.ServeRSS(w, r, req)
		return
	}
	if req.Format == "csv" {
		h.ServeCSV(w, r, req)
		return
	}

	// 3. Perform Aggregation
	resp, err := h.aggregator.Search(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search aggregation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Fetch Instant Answers (if page 1)
	if req.Page == 1 {
		clientIP := h.getClientIP(r)
		ua := r.UserAgent()
		resp.InstantAnswers = h.instantService.FindInstantAnswers(r.Context(), bangParsed.CleanQuery, clientIP, ua)
		for _, ans := range resp.InstantAnswers {
			if ans.Type == "infobox" {
				resp.Infoboxes = append(resp.Infoboxes, ans)
			} else {
				resp.Answers = append(resp.Answers, ans)
			}
		}
		resp.Suggestions = h.suggestService.GetSuggestions(r.Context(), bangParsed.CleanQuery)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")

	faviconResolver := "kagi"
	if cookie, err := r.Cookie("searxgo_favicon_resolver"); err == nil && cookie.Value != "" {
		faviconResolver = cookie.Value
	}
	if fav := r.URL.Query().Get("favicon_resolver"); fav != "" {
		faviconResolver = fav
	}

	data := map[string]interface{}{
		"Response":        resp,
		"TimeRange":       req.TimeRange,
		"Language":        req.Language,
		"SafeSearch":      req.SafeSearch,
		"AllEngines":      engine.DefaultRegistry.GetByCategory(resp.Category),
		"OpenInNewTab":    req.OpenInNewTab,
		"FaviconResolver": faviconResolver,
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "results.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) ServeRSS(w http.ResponseWriter, r *http.Request, req models.SearchRequest) {
	resp, err := h.aggregator.Search(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type RssItem struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		Source      string `xml:"source,omitempty"`
	}

	type RssChannel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Items       []RssItem `xml:"item"`
	}

	type RssFeed struct {
		XMLName xml.Name   `xml:"rss"`
		Version string     `xml:"version,attr"`
		Channel RssChannel `xml:"channel"`
	}

	var items []RssItem
	for _, it := range resp.Results {
		items = append(items, RssItem{
			Title:       it.Title,
			Link:        it.URL,
			Description: it.Content,
			Source:      it.Engine,
		})
	}

	feed := RssFeed{
		Version: "2.0",
		Channel: RssChannel{
			Title:       fmt.Sprintf("%s - SearXGo Search", resp.Query),
			Link:        fmt.Sprintf("http://%s/search?q=%s", r.Host, resp.Query),
			Description: "SearXGo Privacy Metasearch RSS Feed",
			Items:       items,
		},
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(feed)
}

func (h *Handler) ServeCSV(w http.ResponseWriter, r *http.Request, req models.SearchRequest) {
	resp, err := h.aggregator.Search(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"searxgo_%s.csv\"", resp.Query))

	csvWriter := csv.NewWriter(w)
	_ = csvWriter.Write([]string{"Title", "URL", "Content", "Engine", "Category", "Score"})

	for _, it := range resp.Results {
		_ = csvWriter.Write([]string{
			it.Title,
			it.URL,
			it.Content,
			it.Engine,
			string(it.Category),
			fmt.Sprintf("%.2f", it.Score),
		})
	}
	csvWriter.Flush()
}

func (h *Handler) ServeSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		_ = r.ParseForm()

		setPrefCookie := func(name, val string) {
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    val,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				SameSite: http.SameSiteLaxMode,
			})
		}

		if theme := r.PostFormValue("theme"); theme != "" {
			setPrefCookie("searxgo_theme", theme)
		}
		if safe := r.PostFormValue("safesearch"); safe != "" {
			setPrefCookie("searxgo_safesearch", safe)
		}
		if lang := r.PostFormValue("language"); lang != "" {
			setPrefCookie("searxgo_language", lang)
		}
		if doi := r.PostFormValue("doi_resolver"); doi != "" {
			setPrefCookie("searxgo_doi_resolver", doi)
		}
		if inf := r.PostFormValue("infinite_scroll"); inf != "" {
			setPrefCookie("searxgo_infinite_scroll", inf)
		}
		if nt := r.PostFormValue("new_tab"); nt != "" {
			setPrefCookie("searxgo_newtab", nt)
			setPrefCookie("searxgo_new_tab", nt)
		}
		if red := r.PostFormValue("redirects"); red != "" {
			setPrefCookie("searxgo_redirects", red)
		}
		if ac := r.PostFormValue("autocomplete"); ac != "" {
			setPrefCookie("searxgo_autocomplete", ac)
		}
		if tr := r.PostFormValue("tracker_remover"); tr != "" {
			setPrefCookie("searxgo_tracker_remover", tr)
		}
		if fav := r.PostFormValue("favicon_resolver"); fav != "" {
			setPrefCookie("searxgo_favicon_resolver", fav)
		}

		// Enabled engines
		var enabledList []string
		for key, vals := range r.PostForm {
			if strings.HasPrefix(key, "engine_") && len(vals) > 0 && (vals[0] == "1" || vals[0] == "on" || vals[0] == "true") {
				enabledList = append(enabledList, strings.TrimPrefix(key, "engine_"))
			}
		}
		if len(enabledList) > 0 {
			setPrefCookie("searxgo_engines", strings.Join(enabledList, ","))
		}

		http.Redirect(w, r, "/preferences?saved=1", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	categorized := engine.GetCategorizedCatalog()

	safeSearch := "0"
	if cookie, err := r.Cookie("searxgo_safesearch"); err == nil {
		safeSearch = cookie.Value
	}

	data := map[string]interface{}{
		"Categories":   categorized,
		"AllEngines":   engine.FullEngineCatalog,
		"TotalEngines": len(engine.FullEngineCatalog),
		"SafeSearch":   safeSearch,
		"Saved":        r.URL.Query().Get("saved") == "1",
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "settings.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) ServeStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	sysStats := stats.GlobalTracker.GetSystemStats()
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "stats.html", sysStats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) ServeAPIStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	sysStats := stats.GlobalTracker.GetSystemStats()
	_ = json.NewEncoder(w).Encode(sysStats)
}

func (h *Handler) ServeOpenSearch(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	w.Header().Set("Content-Type", "application/opensearchdescription+xml; charset=utf-8")
	data := map[string]string{
		"BaseURL": baseURL,
	}
	if err := h.templates.ExecuteTemplate(w, "opensearch.xml", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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

func (h *Handler) ServeSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	backend := r.URL.Query().Get("autocomplete")
	if backend == "" {
		if cookie, err := r.Cookie("searxgo_autocomplete"); err == nil && cookie.Value != "" {
			backend = cookie.Value
		}
	}
	if backend == "" {
		backend = "all"
	}

	suggestions := h.suggestService.GetSuggestionsWithBackend(r.Context(), q, backend)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(suggestions)
}

func (h *Handler) ServeAutocompleter(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	backend := r.URL.Query().Get("autocomplete")
	if backend == "" {
		if cookie, err := r.Cookie("searxgo_autocomplete"); err == nil && cookie.Value != "" {
			backend = cookie.Value
		}
	}
	if backend == "" {
		backend = "all"
	}

	suggestions := h.suggestService.GetSuggestionsWithBackend(r.Context(), q, backend)

	w.Header().Set("Content-Type", "application/x-suggestions+json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// OpenSearch standard array format: [query, [sug1, sug2, ...]]
	response := []interface{}{
		q,
		suggestions,
	}
	_ = json.NewEncoder(w).Encode(response)
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

func (h *Handler) ServeClearCookies(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		if strings.HasPrefix(cookie.Name, "searx") {
			http.SetCookie(w, &http.Cookie{
				Name:    cookie.Name,
				Value:   "",
				Path:    "/",
				MaxAge:  -1,
			})
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) ServeEngineDescriptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(engine.FullEngineCatalog)
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data: https: http:; font-src 'self' data:; connect-src 'self'; frame-src 'self' https://www.youtube.com https://www.dailymotion.com https://player.vimeo.com https://www.bilibili.com;")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) ServeSplit(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	engineA := r.URL.Query().Get("engine_a")
	if engineA == "" {
		engineA = "duckduckgo"
	}
	engineB := r.URL.Query().Get("engine_b")
	if engineB == "" {
		engineB = "brave"
	}

	var resultsA, resultsB []models.SearchResult
	if q != "" {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			reqA := models.SearchRequest{
				Query:          q,
				Category:       models.CategoryGeneral,
				Page:           1,
				PageSize:       15,
				EnabledEngines: []string{engineA},
			}
			respA, err := h.aggregator.Search(r.Context(), reqA)
			if err == nil && respA != nil {
				resultsA = respA.Results
			}
		}()

		go func() {
			defer wg.Done()
			reqB := models.SearchRequest{
				Query:          q,
				Category:       models.CategoryGeneral,
				Page:           1,
				PageSize:       15,
				EnabledEngines: []string{engineB},
			}
			respB, err := h.aggregator.Search(r.Context(), reqB)
			if err == nil && respB != nil {
				resultsB = respB.Results
			}
		}()

		wg.Wait()
	}

	data := map[string]interface{}{
		"Query":    q,
		"EngineA":  engineA,
		"EngineB":  engineB,
		"ResultsA": resultsA,
		"ResultsB": resultsB,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "split.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) ServeWatchdog(w http.ResponseWriter, r *http.Request) {
	req := h.parseSearchRequest(r)
	if req.Query == "" {
		http.Error(w, "Query parameter 'q' is required for watchdog feed", http.StatusBadRequest)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "json" {
		h.ServeAPI(w, r)
		return
	}

	// Serve as RSS 2.0 Webhook / Notification Feed
	h.ServeRSS(w, r, req)
}

type ReverseEngineItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type ReverseImageResponse struct {
	Success    bool                `json:"success"`
	ImageURL   string              `json:"image_url"`
	Filename   string              `json:"filename"`
	Size       int64               `json:"size"`
	Dimensions string              `json:"dimensions"`
	MimeType   string              `json:"mime_type"`
	Engines    []ReverseEngineItem `json:"engines"`
	Error      string              `json:"error,omitempty"`
}

func (h *Handler) cleanupExpiredImages() {
	h.imageStoreMu.Lock()
	defer h.imageStoreMu.Unlock()
	cutoff := time.Now().Add(-30 * time.Minute)
	for id, img := range h.imageStore {
		if img.CreatedAt.Before(cutoff) {
			delete(h.imageStore, id)
		}
	}
}

func (h *Handler) storeImageBytes(data []byte, filename string) (*TempUploadedImage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}
	if len(data) > 20*1024*1024 {
		return nil, fmt.Errorf("image exceeds maximum size of 20MB")
	}

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		if strings.HasSuffix(strings.ToLower(filename), ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(strings.ToLower(filename), ".webp") {
			contentType = "image/webp"
		} else if strings.HasSuffix(strings.ToLower(filename), ".gif") {
			contentType = "image/gif"
		} else {
			contentType = "image/jpeg"
		}
	}

	width, height := 0, 0
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width = cfg.Width
		height = cfg.Height
	}

	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	id := fmt.Sprintf("img_%d_%s", time.Now().Unix(), hex.EncodeToString(randBytes))

	if filename == "" {
		filename = id + ".jpg"
	}

	item := &TempUploadedImage{
		ID:          id,
		Filename:    filename,
		Data:        data,
		ContentType: contentType,
		Size:        int64(len(data)),
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now(),
	}

	h.cleanupExpiredImages()

	h.imageStoreMu.Lock()
	h.imageStore[id] = item
	h.imageStoreMu.Unlock()

	return item, nil
}

func (h *Handler) ServeTempImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}

	h.imageStoreMu.RLock()
	item, exists := h.imageStore[id]
	h.imageStoreMu.RUnlock()

	if !exists || item == nil {
		http.Error(w, "Image not found or expired", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(item.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Write(item.Data)
}

func (h *Handler) ServeUploadImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
	if err := r.ParseMultipartForm(20 * 1024 * 1024); err != nil {
		http.Error(w, `{"error":"Failed to parse multipart form or image too large"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"No image file provided in 'image' or 'file' field"}`, http.StatusBadRequest)
			return
		}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"Failed to read uploaded image"}`, http.StatusInternalServerError)
		return
	}

	item, err := h.storeImageBytes(data, header.Filename)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8184"
	}
	publicURL := fmt.Sprintf("%s://%s/upload/image/%s", scheme, host, item.ID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"id":         item.ID,
		"image_url":  publicURL,
		"filename":   item.Filename,
		"size":       item.Size,
		"width":      item.Width,
		"height":     item.Height,
		"dimensions": fmt.Sprintf("%dx%d", item.Width, item.Height),
		"mime_type":  item.ContentType,
	})
}

func (h *Handler) buildReverseSearchEngines(targetImageURL string) []ReverseEngineItem {
	escapedURL := url.QueryEscape(targetImageURL)
	return []ReverseEngineItem{
		{
			ID:          "google_lens",
			Name:        "Google Lens",
			URL:         fmt.Sprintf("https://lens.google.com/uploadbyurl?url=%s", escapedURL),
			Icon:        "🔍",
			Description: "Object detection, OCR text extraction, visual similarity & web source matching",
		},
		{
			ID:          "bing_visual",
			Name:        "Bing Visual Search",
			URL:         fmt.Sprintf("https://www.bing.com/images/searchbyimage?cbir=sbi&imageurl=%s", escapedURL),
			Icon:        "🌐",
			Description: "Microsoft visual AI, related products, landmarks and high-res image lookups",
		},
		{
			ID:          "yandex",
			Name:        "Yandex Reverse Images",
			URL:         fmt.Sprintf("https://yandex.com/images/search?rpt=imageview&url=%s", escapedURL),
			Icon:        "🖼️",
			Description: "Unfiltered face matching, duplicate finder & original source indexing",
		},
		{
			ID:          "tineye",
			Name:        "TinEye Reverse Search",
			URL:         fmt.Sprintf("https://tineye.com/search?url=%s", escapedURL),
			Icon:        "🤖",
			Description: "Historical image tracking, modified version tracker, and domain source lookups",
		},
		{
			ID:          "saucenao",
			Name:        "SauceNAO",
			URL:         fmt.Sprintf("https://saucenao.com/search.php?url=%s", escapedURL),
			Icon:        "🎨",
			Description: "Specialized anime, manga, Pixiv, DeviantArt, and digital illustration source finder",
		},
		{
			ID:          "tracemoe",
			Name:        "Trace.moe",
			URL:         fmt.Sprintf("https://trace.moe/?url=%s", escapedURL),
			Icon:        "⚡",
			Description: "Exact anime scene timestamp, episode and title recognition engine",
		},
		{
			ID:          "searxgo_images",
			Name:        "SearXGo Aggregated Search",
			URL:         fmt.Sprintf("/search?q=%s&category=images", escapedURL),
			Icon:        "🪐",
			Description: "Multi-engine aggregated image search across DuckDuckGo, Google, Bing, Unsplash, etc.",
		},
	}
}

func (h *Handler) ServeReverseImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8184"
	}

	var targetImageURL string
	var filename string
	var fileSize int64
	var dimensions string
	var mimeType string

	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
			if err := r.ParseMultipartForm(20 * 1024 * 1024); err == nil {
				file, header, err := r.FormFile("image")
				if err != nil {
					file, header, err = r.FormFile("file")
				}
				if err == nil {
					defer file.Close()
					data, _ := io.ReadAll(file)
					if item, err := h.storeImageBytes(data, header.Filename); err == nil {
						targetImageURL = fmt.Sprintf("%s://%s/upload/image/%s", scheme, host, item.ID)
						filename = item.Filename
						fileSize = item.Size
						dimensions = fmt.Sprintf("%d × %d px", item.Width, item.Height)
						mimeType = item.ContentType
					}
				}
			}
		} else if strings.HasPrefix(contentType, "application/json") {
			var body struct {
				ImageURL    string `json:"image_url"`
				ImageBase64 string `json:"image_base64"`
				Filename    string `json:"filename"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if body.ImageURL != "" {
					targetImageURL = body.ImageURL
					filename = body.Filename
				} else if body.ImageBase64 != "" {
					rawB64 := body.ImageBase64
					if idx := strings.Index(rawB64, ","); idx != -1 {
						rawB64 = rawB64[idx+1:]
					}
					if data, err := base64.StdEncoding.DecodeString(rawB64); err == nil {
						if item, err := h.storeImageBytes(data, body.Filename); err == nil {
							targetImageURL = fmt.Sprintf("%s://%s/upload/image/%s", scheme, host, item.ID)
							filename = item.Filename
							fileSize = item.Size
							dimensions = fmt.Sprintf("%d × %d px", item.Width, item.Height)
							mimeType = item.ContentType
						}
					}
				}
			}
		}
	}

	if targetImageURL == "" {
		targetImageURL = strings.TrimSpace(r.URL.Query().Get("url"))
		if targetImageURL == "" {
			targetImageURL = strings.TrimSpace(r.URL.Query().Get("image_url"))
		}
	}

	if targetImageURL == "" {
		http.Error(w, `{"success":false,"error":"Please provide an image file, base64 data, or an image_url"}`, http.StatusBadRequest)
		return
	}

	if filename == "" {
		parts := strings.Split(targetImageURL, "/")
		if len(parts) > 0 {
			filename = parts[len(parts)-1]
		}
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
		if filename == "" {
			filename = "image.jpg"
		}
	}

	engines := h.buildReverseSearchEngines(targetImageURL)

	resp := ReverseImageResponse{
		Success:    true,
		ImageURL:   targetImageURL,
		Filename:   filename,
		Size:       fileSize,
		Dimensions: dimensions,
		MimeType:   mimeType,
		Engines:    engines,
	}

	json.NewEncoder(w).Encode(resp)
}

// ServeDorks renders the Google Dorking and Recon Suite UI
func (h *Handler) ServeDorks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	categories := dorks.GetAllCategories()

	data := map[string]interface{}{
		"Categories": categories,
		"Target":     target,
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "dorks.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIDorks provides structured JSON output for reconnaissance automation
func (h *Handler) ServeAPIDorks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	categories := dorks.GetAllCategories()

	type apiDorkItem struct {
		dorks.DorkItem
		ScopedQuery string `json:"scoped_query"`
		GoogleURL   string `json:"google_url"`
		SearxgoURL  string `json:"searxgo_url"`
	}

	type apiCategory struct {
		ID          string        `json:"id"`
		Name        string        `json:"name"`
		Icon        string        `json:"icon"`
		Description string        `json:"description"`
		Dorks       []apiDorkItem `json:"dorks"`
	}

	var res []apiCategory
	for _, cat := range categories {
		var dorkItems []apiDorkItem
		for _, d := range cat.Dorks {
			dorkItems = append(dorkItems, apiDorkItem{
				DorkItem:    d,
				ScopedQuery: d.BuildQuery(target),
				GoogleURL:   d.GoogleURL(target),
				SearxgoURL:  d.SearxgoURL(target),
			})
		}
		res = append(res, apiCategory{
			ID:          cat.ID,
			Name:        cat.Name,
			Icon:        cat.Icon,
			Description: cat.Description,
			Dorks:       dorkItems,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"target":     target,
		"categories": res,
	})
}

// ServeScrub renders the EXIF Metadata Stripper & Privacy Cleaner UI
func (h *Handler) ServeScrub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "scrub.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIScrubInspect inspects and extracts EXIF metadata and GPS coordinates
func (h *Handler) ServeAPIScrubInspect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
	if err := r.ParseMultipartForm(25 * 1024 * 1024); err != nil {
		http.Error(w, `{"error":"File too large or invalid multipart form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"No image provided"}`, http.StatusBadRequest)
			return
		}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"Failed to read file"}`, http.StatusInternalServerError)
		return
	}

	report, err := scrub.InspectMetadata(data, header.Filename)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ServeAPIScrubClean purges all EXIF metadata and returns a clean sanitized image
func (h *Handler) ServeAPIScrubClean(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
	if err := r.ParseMultipartForm(25 * 1024 * 1024); err != nil {
		http.Error(w, `{"error":"File too large or invalid multipart form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"No image provided"}`, http.StatusBadRequest)
			return
		}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"Failed to read file"}`, http.StatusInternalServerError)
		return
	}

	cleanBytes, report, err := scrub.CleanImage(data, header.Filename)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check if JSON response was requested or file download
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		report.CleanedDataB64 = base64.StdEncoding.EncodeToString(cleanBytes)
		json.NewEncoder(w).Encode(report)
		return
	}

	cleanFilename := "clean_" + header.Filename
	if !strings.Contains(cleanFilename, ".") {
		cleanFilename += ".jpg"
	}

	w.Header().Set("Content-Type", report.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, cleanFilename))
	w.Header().Set("X-Saved-Percent", fmt.Sprintf("%.1f%%", report.SavedPercent))
	w.Header().Set("X-Tags-Stripped", strconv.Itoa(report.TagsStripped))
	w.Write(cleanBytes)
}

// ServeRecon renders the All-in-One Domain Recon & Security Auditor UI
func (h *Handler) ServeRecon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	data := map[string]interface{}{
		"Target": target,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "recon.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIRecon runs automated security audit on target domain
func (h *Handler) ServeAPIRecon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		http.Error(w, `{"error":"Parameter 'target' is required"}`, http.StatusBadRequest)
		return
	}

	report, err := recon.AuditDomain(r.Context(), target)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ServeReader renders clean, ad-free article reader view
func (h *Handler) ServeReader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	art, err := reader.ExtractArticle(r.Context(), rawURL)
	data := map[string]interface{}{
		"Article":   art,
		"TargetURL": rawURL,
	}
	if err != nil {
		data["Error"] = err.Error()
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "reader.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIReader provides clean article text and metadata in JSON format
func (h *Handler) ServeAPIReader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Error(w, `{"error":"Parameter 'url' is required"}`, http.StatusBadRequest)
		return
	}

	art, err := reader.ExtractArticle(r.Context(), rawURL)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(art)
}

// ServeSherlock renders the interactive OSINT Sherlock username reconnaissance workspace
func (h *Handler) ServeSherlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		username = strings.TrimSpace(r.URL.Query().Get("u"))
	}
	if username == "" {
		username = strings.TrimSpace(r.URL.Query().Get("target"))
	}

	platforms := instant.GetSherlockPlatforms("sample")

	data := map[string]interface{}{
		"Username":  username,
		"Platforms": platforms,
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "sherlock.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPISherlock scans social and developer platforms for username presence
func (h *Handler) ServeAPISherlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		username = strings.TrimSpace(r.URL.Query().Get("u"))
	}
	if username == "" {
		username = strings.TrimSpace(r.URL.Query().Get("target"))
	}

	if r.Method == http.MethodPost && username == "" {
		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			username = strings.TrimSpace(req.Username)
		}
	}

	if username == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Parameter 'username' is required"})
		return
	}

	cleanUser, ok := instant.CleanUsername(username)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username format (3-32 alphanumeric/underscore/dot/dash characters)"})
		return
	}

	report := instant.ScanUsernameProfiles(r.Context(), cleanUser, 3500*time.Millisecond)
	json.NewEncoder(w).Encode(report)
}

// ServeThreat renders the URL Threat Intelligence & Sandbox scanner workspace
func (h *Handler) ServeThreat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	targetURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		targetURL = strings.TrimSpace(r.URL.Query().Get("u"))
	}

	data := map[string]interface{}{
		"TargetURL": targetURL,
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "threat.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIThreat runs URL threat intelligence analysis and returns structured JSON report
func (h *Handler) ServeAPIThreat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	targetURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		targetURL = strings.TrimSpace(r.URL.Query().Get("u"))
	}

	if r.Method == http.MethodPost && targetURL == "" {
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			targetURL = strings.TrimSpace(req.URL)
		}
	}

	if targetURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Parameter 'url' is required"})
		return
	}

	report := threat.ScanURL(r.Context(), targetURL)
	json.NewEncoder(w).Encode(report)
}

// ServeGraph renders the interactive keyword/entity knowledge graph workspace
func (h *Handler) ServeGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	data := map[string]interface{}{
		"Query": query,
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "graph.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIGraph extracts entity/domain/keyword nodes from search results and returns graph JSON
func (h *Handler) ServeAPIGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Parameter 'q' is required"})
		return
	}

	searchReq := models.SearchRequest{
		Query:    query,
		Category: models.CategoryGeneral,
		Page:     1,
		PageSize: 20,
	}

	resp, err := h.aggregator.Search(r.Context(), searchReq)
	if err != nil || resp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Search failed"})
		return
	}

	type GraphNode struct {
		ID    string  `json:"id"`
		Label string  `json:"label"`
		Type  string  `json:"type"`
		URL   string  `json:"url,omitempty"`
		Score float64 `json:"score"`
	}
	type GraphEdge struct {
		Source string  `json:"source"`
		Target string  `json:"target"`
		Weight float64 `json:"weight"`
		Label  string  `json:"label,omitempty"`
	}
	type GraphData struct {
		Query string      `json:"query"`
		Nodes []GraphNode `json:"nodes"`
		Edges []GraphEdge `json:"edges"`
		Total int         `json:"total"`
	}

	nodes := []GraphNode{{
		ID:    "root",
		Label: query,
		Type:  "query",
		Score: 1.0,
	}}
	edges := []GraphEdge{}
	domainsSeen := map[string]bool{}
	keywordsSeen := map[string]bool{}

	stopWords := map[string]bool{
		"this": true, "that": true, "with": true, "from": true,
		"have": true, "will": true, "your": true, "more": true,
		"about": true, "into": true, "than": true, "they": true,
		"were": true, "been": true, "when": true, "also": true,
		"what": true, "which": true, "there": true, "their": true,
		"would": true, "could": true, "should": true, "after": true,
		"where": true, "while": true, "these": true, "some": true,
	}

	for i, result := range resp.Results {
		if i >= 15 {
			break
		}

		parsedU, uErr := url.Parse(result.URL)
		domainNode := ""
		if uErr == nil && parsedU.Host != "" {
			domain := strings.TrimPrefix(strings.ToLower(parsedU.Host), "www.")
			if !domainsSeen[domain] {
				domainNode = "domain:" + domain
				nodes = append(nodes, GraphNode{
					ID:    domainNode,
					Label: domain,
					Type:  "domain",
					URL:   parsedU.Scheme + "://" + parsedU.Host,
					Score: result.Score,
				})
				domainsSeen[domain] = true
				edges = append(edges, GraphEdge{
					Source: "root",
					Target: domainNode,
					Weight: result.Score + 0.1,
					Label:  "source",
				})
			} else {
				domainNode = "domain:" + domain
			}
		}

		titleWords := strings.Fields(strings.ToLower(result.Title))
		for _, word := range titleWords {
			word = strings.Trim(word, ".,!?;:()[]{}\"'")
			if len(word) < 4 || stopWords[word] {
				continue
			}
			if keywordsSeen[word] {
				if domainNode != "" {
					edges = append(edges, GraphEdge{
						Source: domainNode,
						Target: "kw:" + word,
						Weight: 0.3,
					})
				}
				continue
			}
			if len(nodes) >= 80 {
				break
			}
			keywordsSeen[word] = true
			kwNode := "kw:" + word
			nodes = append(nodes, GraphNode{
				ID:    kwNode,
				Label: word,
				Type:  "keyword",
				Score: 0.2,
			})
			if domainNode != "" {
				edges = append(edges, GraphEdge{
					Source: domainNode,
					Target: kwNode,
					Weight: 0.4,
				})
			}
		}
	}

	json.NewEncoder(w).Encode(GraphData{
		Query: query,
		Nodes: nodes,
		Edges: edges,
		Total: len(nodes),
	})
}

// ServeAPIGogglesValidate validates and normalizes a domain block/boost list
func (h *Handler) ServeAPIGogglesValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Blocklist []string `json:"blocklist"`
		Boostlist []string `json:"boostlist"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON body"})
		return
	}

	cleanDomain := func(d string) string {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		d = strings.TrimPrefix(d, "www.")
		d = strings.Split(d, "/")[0]
		return d
	}

	cleanedBlock := []string{}
	for _, d := range req.Blocklist {
		if c := cleanDomain(d); c != "" {
			cleanedBlock = append(cleanedBlock, c)
		}
	}

	cleanedBoost := []string{}
	for _, d := range req.Boostlist {
		if c := cleanDomain(d); c != "" {
			cleanedBoost = append(cleanedBoost, c)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocklist": cleanedBlock,
		"boostlist": cleanedBoost,
		"count":     len(cleanedBlock) + len(cleanedBoost),
	})
}

// ==============================================================================
// 🕰️ Internet Time Machine & Wayback Machine Handlers
// ==============================================================================

func (h *Handler) ServeArchive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	targetURL := r.URL.Query().Get("url")
	data := map[string]interface{}{
		"TargetURL": targetURL,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "archive.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIArchive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	targetURL := r.URL.Query().Get("url")
	if strings.TrimSpace(targetURL) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url parameter is required"})
		return
	}

	report, err := h.archiveClient.GetSnapshots(r.Context(), targetURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// ⚡ Tech Stack & Security Header Inspector Handlers
// ==============================================================================

func (h *Handler) ServeTech(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	targetURL := r.URL.Query().Get("url")
	data := map[string]interface{}{
		"TargetURL": targetURL,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "tech.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPITech(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	targetURL := r.URL.Query().Get("url")
	if strings.TrimSpace(targetURL) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url parameter is required"})
		return
	}

	report, err := h.techInspector.Analyze(r.Context(), targetURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// 🌐 Subdomain Enumeration (CT Logs) Handlers
// ==============================================================================

func (h *Handler) ServeSubdomains(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	domain := r.URL.Query().Get("domain")
	data := map[string]interface{}{
		"Domain": domain,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "subdomains.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPISubdomains(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	domain := r.URL.Query().Get("domain")
	if strings.TrimSpace(domain) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "domain parameter is required"})
		return
	}

	report, err := h.subFinder.Enumerate(r.Context(), domain)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// 🌐 DNS Propagation Handlers
// ==============================================================================

func (h *Handler) ServeDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	domain := r.URL.Query().Get("domain")
	data := map[string]interface{}{
		"Domain": domain,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "dns.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	domain := r.URL.Query().Get("domain")
	recordType := r.URL.Query().Get("type")
	if strings.TrimSpace(domain) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "domain parameter is required"})
		return
	}

	report, err := h.dnsService.CheckPropagation(r.Context(), domain, recordType)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// ⚡ Is It Down & Port Ping Tester Handlers
// ==============================================================================

func (h *Handler) ServePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	target := r.URL.Query().Get("target")
	data := map[string]interface{}{
		"Target": target,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "ping.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	target := r.URL.Query().Get("target")
	if strings.TrimSpace(target) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "target parameter is required"})
		return
	}

	report, err := h.pingService.Check(r.Context(), target)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// 🔓 Paywall Bypass Handlers
// ==============================================================================

func (h *Handler) ServeBypass(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	targetURL := r.URL.Query().Get("url")
	data := map[string]interface{}{
		"TargetURL": targetURL,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "bypass.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIBypass(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	targetURL := r.URL.Query().Get("url")
	if strings.TrimSpace(targetURL) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url parameter is required"})
		return
	}

	article, err := h.bypassService.ExtractClean(r.Context(), targetURL)
	if err != nil {
		// Even if direct extraction fails, return the multi-mirror links!
		mirrors := bypass.GenerateMirrors(targetURL)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"target_url": targetURL,
			"mirrors":    mirrors,
			"error":      err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(article)
}

// ==============================================================================
// 📥 Media Extractor Handlers
// ==============================================================================

func (h *Handler) ServeMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	targetURL := r.URL.Query().Get("url")
	data := map[string]interface{}{
		"TargetURL": targetURL,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "media.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	targetURL := r.URL.Query().Get("url")
	if strings.TrimSpace(targetURL) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url parameter is required"})
		return
	}

	result, err := h.mediaService.Extract(r.Context(), targetURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(result)
}

// ==============================================================================
// 📱 QR Studio Handlers
// ==============================================================================

func (h *Handler) ServeQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "qr.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIQRWiFi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ssid := r.URL.Query().Get("ssid")
	pass := r.URL.Query().Get("pass")
	enc := r.URL.Query().Get("enc")
	hidden := r.URL.Query().Get("hidden") == "true"

	payload := qr.FormatWiFiPayload(qr.WiFiConfig{
		SSID:       ssid,
		Password:   pass,
		Encryption: enc,
		Hidden:     hidden,
	})

	json.NewEncoder(w).Encode(map[string]string{
		"payload": payload,
	})
}

// ==============================================================================
// 📰 World News Pulse Handlers
// ==============================================================================

func (h *Handler) ServeNewsHub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "newshub.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPINewsHub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	topic := r.URL.Query().Get("topic")
	region := r.URL.Query().Get("region")
	query := r.URL.Query().Get("q")

	articles, err := h.newsHubService.GetNews(r.Context(), topic, region, query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"topic":    topic,
		"region":   region,
		"count":    len(articles),
		"articles": articles,
	})
}

// ==============================================================================
// 🌤️ Live Weather Radar & Forecast Handlers
// ==============================================================================

func (h *Handler) ServeWeather(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "weather.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPIWeather(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	query := r.URL.Query().Get("q")

	if latStr != "" && lonStr != "" {
		lat, err1 := strconv.ParseFloat(latStr, 64)
		lon, err2 := strconv.ParseFloat(lonStr, 64)
		if err1 == nil && err2 == nil {
			report, err := h.weatherService.GetWeatherByCoords(r.Context(), lat, lon, nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(report)
			return
		}
	}

	if query == "" {
		query = "Jakarta"
	}

	report, err := h.weatherService.GetWeatherByQuery(r.Context(), query)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// 💱 Live Currency & Crypto Exchange Handlers
// ==============================================================================

func (h *Handler) ServeCurrency(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "currency.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPICurrencyConvert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	amountStr := r.URL.Query().Get("amount")

	if from == "" {
		from = "USD"
	}
	if to == "" {
		to = "IDR"
	}
	amount := 1.0
	if amountStr != "" {
		if val, err := strconv.ParseFloat(amountStr, 64); err == nil {
			amount = val
		}
	}

	res, err := h.currencyService.Convert(from, to, amount)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(res)
}

func (h *Handler) ServeAPICurrencyCrypto(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cryptos := h.currencyService.GetTopCryptos()
	json.NewEncoder(w).Encode(cryptos)
}

func (h *Handler) ServeAPICurrencyCurrencies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currency.CurrencyList)
}

func (h *Handler) ServeAPICurrencyRates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rates := h.currencyService.GetRatesSnapshot()
	json.NewEncoder(w).Encode(rates)
}











