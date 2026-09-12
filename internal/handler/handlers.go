package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"

	"searxgo/internal/aggregator"
	"searxgo/internal/bangs"
	"searxgo/internal/config"
	"searxgo/internal/engine"
	"searxgo/internal/instant"
	"searxgo/internal/models"
	"searxgo/internal/proxy"
	"searxgo/internal/stats"
	"searxgo/web"
)

type Handler struct {
	cfg            *config.Config
	aggregator     *aggregator.Aggregator
	instantService *instant.InstantService
	suggestService *SuggestService
	imageProxy     *proxy.ImageProxy
	limiter        *RateLimiter
	templates      *template.Template
}

func NewHandler(cfg *config.Config, agg *aggregator.Aggregator) (*Handler, error) {
	tmplFuncs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"safe": func(s string) template.HTML { return template.HTML(s) },
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

	// Privacy Proxy
	mux.HandleFunc("GET /proxy/image", h.imageProxy.ServeHTTP)
	mux.HandleFunc("GET /image_proxy", h.imageProxy.ServeHTTP) // SearXNG official alias

	// Static Assets
	fileServer := http.FileServer(http.FS(web.StaticFS))
	mux.Handle("GET /static/", fileServer)
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

	return models.SearchRequest{
		Query:           rawQ,
		RawQuery:        rawQ,
		Category:        cat,
		Page:            page,
		PageSize:        pageSize,
		Country:         country,
		Region:          region,
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
		"Categories": categorized,
		"AllEngines": engine.FullEngineCatalog,
		"SafeSearch": safeSearch,
		"Saved":      r.URL.Query().Get("saved") == "1",
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


