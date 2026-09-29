package handler

import (
	"bytes"
	"net/http"
	"strings"

	"searxgo/internal/engine"
	"searxgo/internal/models"
	"searxgo/internal/stats"
)

// ServeSettings manages viewing and persisting user configuration via cookies
func (h *Handler) ServeSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			_ = r.ParseMultipartForm(10 << 20)
		} else {
			_ = r.ParseForm()
		}

		getFormVals := func(key string) []string {
			if r.MultipartForm != nil && r.MultipartForm.Value != nil {
				if vals, ok := r.MultipartForm.Value[key]; ok {
					return vals
				}
			}
			if r.PostForm != nil {
				if vals, ok := r.PostForm[key]; ok {
					return vals
				}
			}
			return r.Form[key]
		}
		getFormVal := func(key string) string {
			vals := getFormVals(key)
			if len(vals) > 0 {
				return vals[0]
			}
			return ""
		}

		setPrefCookie := func(name, val string) {
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    val,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				SameSite: http.SameSiteLaxMode,
			})
		}

		if theme := getFormVal("theme"); theme != "" {
			setPrefCookie("searxgo_theme", theme)
		}
		if safe := getFormVal("safesearch"); safe != "" {
			setPrefCookie("searxgo_safesearch", safe)
		}
		if lang := getFormVal("language"); lang != "" {
			setPrefCookie("searxgo_language", lang)
		}
		if ac := getFormVal("autocomplete"); ac != "" {
			setPrefCookie("searxgo_autocomplete", ac)
		}
		if fav := getFormVal("favicon_resolver"); fav != "" {
			setPrefCookie("searxgo_favicon_resolver", fav)
		}
		if doi := getFormVal("doi_resolver"); doi != "" {
			setPrefCookie("searxgo_doi_resolver", doi)
		}

		// Checkboxes: unchecked checkboxes in HTML are omitted from form body
		if v := getFormVal("new_tab"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_newtab", "true")
			setPrefCookie("searxgo_new_tab", "true")
		} else {
			setPrefCookie("searxgo_newtab", "false")
			setPrefCookie("searxgo_new_tab", "false")
		}

		if v := getFormVal("infinite_scroll"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_infinite_scroll", "true")
		} else {
			setPrefCookie("searxgo_infinite_scroll", "false")
		}

		if v := getFormVal("redirects"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_redirects", "true")
		} else {
			setPrefCookie("searxgo_redirects", "false")
		}

		if v := getFormVal("proxy"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_proxy", "true")
		} else {
			setPrefCookie("searxgo_proxy", "false")
		}

		if v := getFormVal("tracker_remover"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_tracker_remover", "true")
		} else {
			setPrefCookie("searxgo_tracker_remover", "false")
		}

		if v := getFormVal("center_alignment"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_center_alignment", "true")
		} else {
			setPrefCookie("searxgo_center_alignment", "false")
		}

		if m := getFormVal("method"); m != "" {
			setPrefCookie("searxgo_method", m)
		}
		if hk := getFormVal("hotkeys"); hk != "" {
			setPrefCookie("searxgo_hotkeys", hk)
		}
		if uf := getFormVal("url_formatting"); uf != "" {
			setPrefCookie("searxgo_url_formatting", uf)
		}
		if uil := getFormVal("ui_locale"); uil != "" {
			setPrefCookie("searxgo_ui_locale", uil)
		}
		if tok := getFormVal("engine_tokens"); tok != "" {
			setPrefCookie("searxgo_tokens", tok)
		}

		if v := getFormVal("unit_converter"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_unit_converter", "true")
		} else {
			setPrefCookie("searxgo_unit_converter", "false")
		}

		if v := getFormVal("doi_rewrite"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_doi_rewrite", "true")
		} else {
			setPrefCookie("searxgo_doi_rewrite", "false")
		}

		if v := getFormVal("cached_links"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_cached_links", "true")
		} else {
			setPrefCookie("searxgo_cached_links", "false")
		}

		if v := getFormVal("spam_guard"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_spam_guard", "true")
		} else {
			setPrefCookie("searxgo_spam_guard", "false")
		}

		if v := getFormVal("search_on_category"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_search_on_cat", "true")
		} else {
			setPrefCookie("searxgo_search_on_cat", "false")
		}

		if v := getFormVal("query_in_title"); v == "true" || v == "on" || v == "1" {
			setPrefCookie("searxgo_query_in_title", "true")
		} else {
			setPrefCookie("searxgo_query_in_title", "false")
		}

		// Categories
		defaultCatsRaw := getFormVals("default_categories")
		var defaultCats []string
		for _, cat := range defaultCatsRaw {
			trimmed := strings.ToLower(strings.TrimSpace(cat))
			if trimmed != "" {
				defaultCats = append(defaultCats, trimmed)
			}
		}
		if len(defaultCats) > 0 {
			setPrefCookie("searxgo_categories", strings.Join(defaultCats, ","))
		} else {
			setPrefCookie("searxgo_categories", "general")
		}

		// Enabled engines
		var enabledList []string
		getAllKeysAndVals := func() map[string][]string {
			res := make(map[string][]string)
			if r.MultipartForm != nil && r.MultipartForm.Value != nil {
				for k, v := range r.MultipartForm.Value {
					res[k] = v
				}
			}
			if r.PostForm != nil {
				for k, v := range r.PostForm {
					res[k] = v
				}
			}
			for k, v := range r.Form {
				if _, ok := res[k]; !ok {
					res[k] = v
				}
			}
			return res
		}
		allFormFields := getAllKeysAndVals()
		for key, vals := range allFormFields {
			if strings.HasPrefix(key, "engine_") && len(vals) > 0 {
				engineID := vals[0]
				if engineID != "" && engineID != "0" && engineID != "false" {
					enabledList = append(enabledList, engineID)
				}
			}
		}
		if len(enabledList) > 0 {
			setPrefCookie("searxgo_engines", strings.Join(enabledList, ","))
		}

		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<div id="save-feedback" style="background: var(--accent-emerald); color:#000; font-weight:600; padding: 0.75rem 1rem; border-radius: var(--radius-md); margin-bottom: 1.5rem; text-align:center; animation: fadeIn 0.3s ease;">
      ✓ Preferences successfully saved to local browser cookies (HTMX)!
    </div>`))
			return
		}

		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success": true, "message": "Preferences saved successfully"}`))
			return
		}

		http.Redirect(w, r, "/settings?saved=1", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	getCookieVal := func(name, fallback string) string {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return cookie.Value
		}
		return fallback
	}

	theme := getCookieVal("searxgo_theme", "dark")
	lang := getCookieVal("searxgo_language", "en")
	safeSearch := getCookieVal("searxgo_safesearch", "0")
	autocomplete := getCookieVal("searxgo_autocomplete", "all")
	faviconResolver := getCookieVal("searxgo_favicon_resolver", "kagi")
	doiResolver := getCookieVal("searxgo_doi_resolver", "oadoi.org")
	newTab := getCookieVal("searxgo_newtab", "true") == "true"
	infiniteScroll := getCookieVal("searxgo_infinite_scroll", "false") == "true"
	redirects := getCookieVal("searxgo_redirects", "true") != "false"
	proxy := getCookieVal("searxgo_proxy", "true") != "false"
	trackerRemover := getCookieVal("searxgo_tracker_remover", "true") != "false"
	centerAlignment := getCookieVal("searxgo_center_alignment", "false") == "true"
	method := getCookieVal("searxgo_method", "GET")
	hotkeys := getCookieVal("searxgo_hotkeys", "vim")
	urlFormatting := getCookieVal("searxgo_url_formatting", "pretty")
	uiLocale := getCookieVal("searxgo_ui_locale", "en")
	engineTokens := getCookieVal("searxgo_tokens", "")
	unitConverter := getCookieVal("searxgo_unit_converter", "true") != "false"
	doiRewrite := getCookieVal("searxgo_doi_rewrite", "true") != "false"
	cachedLinks := getCookieVal("searxgo_cached_links", "true") != "false"
	spamGuard := getCookieVal("searxgo_spam_guard", "true") != "false"
	searchOnCategory := getCookieVal("searxgo_search_on_cat", "true") != "false"
	queryInTitle := getCookieVal("searxgo_query_in_title", "true") != "false"

	enginesCookie := getCookieVal("searxgo_engines", "")
	enabledEnginesMap := make(map[string]bool)
	hasCustomEngines := false
	if enginesCookie != "" {
		hasCustomEngines = true
		for _, eng := range strings.Split(enginesCookie, ",") {
			if t := strings.TrimSpace(eng); t != "" {
				enabledEnginesMap[t] = true
			}
		}
	}

	catsCookie := getCookieVal("searxgo_categories", "general")
	enabledCatsMap := make(map[string]bool)
	for _, c := range strings.Split(catsCookie, ",") {
		if t := strings.ToLower(strings.TrimSpace(c)); t != "" {
			enabledCatsMap[t] = true
		}
	}

	categorized := engine.GetCategorizedCatalog()

	sysStats := stats.GlobalTracker.GetSystemStats()
	engineStatsMap := make(map[string]models.EngineStatItem)
	for _, item := range sysStats.EngineStats {
		engineStatsMap[item.Name] = item
		engineStatsMap[strings.ToLower(item.Name)] = item
	}

	data := map[string]interface{}{
		"Categories":        categorized,
		"CategoryList":      DefaultCategoryList,
		"EngineStats":       engineStatsMap,
		"AllEngines":        engine.FullEngineCatalog,
		"TotalEngines":      len(engine.FullEngineCatalog),
		"Theme":             theme,
		"Language":          lang,
		"SafeSearch":        safeSearch,
		"Autocomplete":      autocomplete,
		"FaviconResolver":   faviconResolver,
		"DoiResolver":       doiResolver,
		"NewTab":            newTab,
		"InfiniteScroll":    infiniteScroll,
		"Redirects":         redirects,
		"Proxy":             proxy,
		"TrackerRemover":    trackerRemover,
		"CenterAlignment":   centerAlignment,
		"Method":            method,
		"Hotkeys":           hotkeys,
		"URLFormatting":     urlFormatting,
		"UILocale":          uiLocale,
		"EngineTokens":      engineTokens,
		"UnitConverter":     unitConverter,
		"DoiRewrite":        doiRewrite,
		"CachedLinks":       cachedLinks,
		"SpamGuard":         spamGuard,
		"SearchOnCategory":  searchOnCategory,
		"QueryInTitle":      queryInTitle,
		"EnabledEngines":    enabledEnginesMap,
		"HasCustomEngines":  hasCustomEngines,
		"EnabledCategories": enabledCatsMap,
		"Saved":             r.URL.Query().Get("saved") == "1",
	}

	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "settings.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeClearCookies clears all user preference cookies
func (h *Handler) ServeClearCookies(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		if strings.HasPrefix(cookie.Name, "searx") {
			http.SetCookie(w, &http.Cookie{
				Name:   cookie.Name,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
