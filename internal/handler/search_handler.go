package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"searxgo/internal/bangs"
	"searxgo/internal/engine"
	"searxgo/internal/models"
)

// ServeIndex renders the home search interface
func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	catsCookie := "general"
	if cookie, err := r.Cookie("searxgo_categories"); err == nil && cookie.Value != "" {
		catsCookie = cookie.Value
	}
	catsList := strings.Split(catsCookie, ",")
	defaultCat := "general"
	enabledCatsMap := make(map[string]bool)
	for i, c := range catsList {
		t := strings.TrimSpace(c)
		if t != "" {
			if i == 0 {
				defaultCat = t
			}
			enabledCatsMap[t] = true
		}
	}
	if defaultCat == "" {
		defaultCat = "general"
	}
	enabledCatsMap[defaultCat] = true

	data := map[string]interface{}{
		"Engines":           engine.DefaultRegistry.GetEngineInfos(),
		"DefaultCategory":   defaultCat,
		"DefaultCategories": enabledCatsMap,
		"CategoryList":      DefaultCategoryList,
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
	} else if catStr != "" {
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
		default:
			cat = models.CategoryGeneral
		}
	} else {
		// Category not explicitly provided in URL or POST form, fallback to default_categories cookie
		if cookie, err := r.Cookie("searxgo_categories"); err == nil && cookie.Value != "" {
			firstCat := strings.TrimSpace(strings.Split(cookie.Value, ",")[0])
			switch strings.ToLower(firstCat) {
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
			default:
				cat = models.CategoryGeneral
			}
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

	// Infinite scroll preference
	infiniteScroll := false
	if cookie, err := r.Cookie("searxgo_infinite_scroll"); err == nil && cookie.Value == "true" {
		infiniteScroll = true
	}
	if getParam("infinite_scroll") == "1" || getParam("infinite_scroll") == "true" {
		infiniteScroll = true
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
		InfiniteScroll:  infiniteScroll,
		RemoveTrackers:  removeTrackers,
		Autocomplete:    autocomplete,
	}
}

func (h *Handler) getClientIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		if ip := r.Header.Get(header); ip != "" {
			parts := strings.Split(ip, ",")
			clean := strings.TrimSpace(parts[0])
			if clean != "" {
				return clean
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// ServeSearch performs full privacy search aggregation and formats the output
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

	queryInTitle := true
	if cookie, err := r.Cookie("searxgo_query_in_title"); err == nil && cookie.Value == "false" {
		queryInTitle = false
	}

	data := map[string]interface{}{
		"Response":        resp,
		"TimeRange":       req.TimeRange,
		"Language":        req.Language,
		"SafeSearch":      req.SafeSearch,
		"AllEngines":      engine.DefaultRegistry.GetByCategory(resp.Category),
		"OpenInNewTab":    req.OpenInNewTab,
		"InfiniteScroll":  req.InfiniteScroll,
		"FaviconResolver": faviconResolver,
		"QueryInTitle":    queryInTitle,
		"CategoryList":    DefaultCategoryList,
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

func (h *Handler) ServeEngineDescriptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(engine.FullEngineCatalog)
}
