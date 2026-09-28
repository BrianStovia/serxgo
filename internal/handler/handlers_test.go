package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"searxgo/internal/aggregator"
	"searxgo/internal/config"
	"searxgo/internal/engine"
	"searxgo/internal/models"
)

func TestHTTPHandlers(t *testing.T) {
	cfg := &config.Config{
		Timeout: 3 * time.Second,
	}
	reg := engine.NewRegistry()
	reg.Register(engine.NewWikipediaEngine())
	agg := aggregator.NewAggregator(reg, cfg.Timeout)

	h, err := NewHandler(cfg, agg)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 1. Test Index /
	recIndex := httptest.NewRecorder()
	reqIndex := httptest.NewRequest("GET", "/", nil)
	mux.ServeHTTP(recIndex, reqIndex)

	if recIndex.Code != http.StatusOK {
		t.Errorf("GET / returned status %d; want %d", recIndex.Code, http.StatusOK)
	}

	// 2. Test OpenSearch /opensearch.xml
	recOS := httptest.NewRecorder()
	reqOS := httptest.NewRequest("GET", "/opensearch.xml", nil)
	mux.ServeHTTP(recOS, reqOS)

	if recOS.Code != http.StatusOK {
		t.Errorf("GET /opensearch.xml returned status %d; want %d", recOS.Code, http.StatusOK)
	}

	// 3. Test API Search /api/search without query
	recAPI := httptest.NewRecorder()
	reqAPI := httptest.NewRequest("GET", "/api/search", nil)
	mux.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusBadRequest {
		t.Errorf("GET /api/search without q returned status %d; want %d", recAPI.Code, http.StatusBadRequest)
	}

	// 4. Test Metrics /metrics (Prometheus OpenMetrics)
	recMetrics := httptest.NewRecorder()
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	mux.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Errorf("GET /metrics returned status %d; want %d", recMetrics.Code, http.StatusOK)
	}
	if !strings.Contains(recMetrics.Body.String(), "searxng_searches_total") {
		t.Errorf("GET /metrics body missing 'searxng_searches_total'")
	}

	// 5. Test Config /config
	recConfig := httptest.NewRecorder()
	reqConfig := httptest.NewRequest("GET", "/config", nil)
	mux.ServeHTTP(recConfig, reqConfig)

	if recConfig.Code != http.StatusOK {
		t.Errorf("GET /config returned status %d; want %d", recConfig.Code, http.StatusOK)
	}

	// 6. Test Healthz /healthz
	recHealth := httptest.NewRecorder()
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	mux.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Errorf("GET /healthz returned status %d; want %d", recHealth.Code, http.StatusOK)
	}

	// 7. Test Split Screen /split
	recSplit := httptest.NewRecorder()
	reqSplit := httptest.NewRequest("GET", "/split?q=test", nil)
	mux.ServeHTTP(recSplit, reqSplit)

	if recSplit.Code != http.StatusOK {
		t.Errorf("GET /split returned status %d; want %d", recSplit.Code, http.StatusOK)
	}

	// 8. Test Watchdog /watchdog
	recWatch := httptest.NewRecorder()
	reqWatch := httptest.NewRequest("GET", "/watchdog?q=golang", nil)
	mux.ServeHTTP(recWatch, reqWatch)

	if recWatch.Code != http.StatusOK {
		t.Errorf("GET /watchdog returned status %d; want %d", recWatch.Code, http.StatusOK)
	}
}

func TestReverseImageSearch(t *testing.T) {
	cfg := &config.Config{
		Timeout: 3 * time.Second,
	}
	reg := engine.NewRegistry()
	agg := aggregator.NewAggregator(reg, cfg.Timeout)

	h, err := NewHandler(cfg, agg)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 1. Test GET /api/reverse-image with URL parameter
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/api/reverse-image?url=https://images.unsplash.com/photo-1542291026-7eec264c27ff", nil)
	mux.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("GET /api/reverse-image returned status %d; want %d", rec1.Code, http.StatusOK)
	}

	var resp1 ReverseImageResponse
	if err := json.NewDecoder(rec1.Body).Decode(&resp1); err != nil {
		t.Fatalf("failed to decode reverse image response: %v", err)
	}
	if !resp1.Success || len(resp1.Engines) == 0 {
		t.Errorf("expected success and non-empty engines, got success=%v, engines=%d", resp1.Success, len(resp1.Engines))
	}

	// Verify Google Lens, Bing, Yandex, TinEye, SauceNAO, Trace.moe are included
	engineIDs := make(map[string]bool)
	for _, eng := range resp1.Engines {
		engineIDs[eng.ID] = true
	}
	for _, expectedID := range []string{"google_lens", "bing_visual", "yandex", "tineye", "saucenao", "tracemoe", "searxgo_images"} {
		if !engineIDs[expectedID] {
			t.Errorf("expected engine %s in reverse search engines, but not found", expectedID)
		}
	}

	// 2. Test POST /upload/image multipart form upload
	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)
	// Sample 1x1 transparent PNG bytes
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	part, err := writer.CreateFormFile("image", "sample.png")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(pngBytes)
	writer.Close()

	recUpload := httptest.NewRecorder()
	reqUpload := httptest.NewRequest("POST", "/upload/image", &bodyBuf)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	mux.ServeHTTP(recUpload, reqUpload)

	if recUpload.Code != http.StatusOK {
		t.Errorf("POST /upload/image returned status %d; want %d; body: %s", recUpload.Code, http.StatusOK, recUpload.Body.String())
	}

	var uploadResp struct {
		Success  bool   `json:"success"`
		ID       string `json:"id"`
		ImageURL string `json:"image_url"`
	}
	if err := json.NewDecoder(recUpload.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	if !uploadResp.Success || uploadResp.ID == "" {
		t.Errorf("expected success upload and valid ID, got success=%v, id=%s", uploadResp.Success, uploadResp.ID)
	}

	// 3. Test GET /upload/image/{id}
	recServeImg := httptest.NewRecorder()
	reqServeImg := httptest.NewRequest("GET", "/upload/image/"+uploadResp.ID, nil)
	mux.ServeHTTP(recServeImg, reqServeImg)

	if recServeImg.Code != http.StatusOK {
		t.Errorf("GET /upload/image/%s returned status %d; want %d", uploadResp.ID, recServeImg.Code, http.StatusOK)
	}
	if recServeImg.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png content-type, got %s", recServeImg.Header().Get("Content-Type"))
	}

	// 4. Test GET /sherlock UI
	recSherlock := httptest.NewRecorder()
	reqSherlock := httptest.NewRequest("GET", "/sherlock?username=torvalds", nil)
	mux.ServeHTTP(recSherlock, reqSherlock)
	if recSherlock.Code != http.StatusOK {
		t.Errorf("GET /sherlock returned %d, want 200", recSherlock.Code)
	}
	if !strings.Contains(recSherlock.Body.String(), "OSINT Sherlock Workspace") {
		t.Errorf("expected page to contain OSINT Sherlock title")
	}

	// 5. Test GET /api/sherlock without param
	recAPISherlockEmpty := httptest.NewRecorder()
	reqAPISherlockEmpty := httptest.NewRequest("GET", "/api/sherlock", nil)
	mux.ServeHTTP(recAPISherlockEmpty, reqAPISherlockEmpty)
	if recAPISherlockEmpty.Code != http.StatusBadRequest {
		t.Errorf("GET /api/sherlock without username returned %d, want 400", recAPISherlockEmpty.Code)
	}

	// 6. Test GET /api/sherlock with invalid handle
	recAPISherlockBad := httptest.NewRecorder()
	reqAPISherlockBad := httptest.NewRequest("GET", "/api/sherlock?username=!invalid", nil)
	mux.ServeHTTP(recAPISherlockBad, reqAPISherlockBad)
	if recAPISherlockBad.Code != http.StatusBadRequest {
		t.Errorf("GET /api/sherlock with invalid user returned %d, want 400", recAPISherlockBad.Code)
	}

	// 7. Test POST /settings and GET /settings with cookies
	form := strings.NewReader("theme=cyberpunk&safesearch=1&language=id&engine_google__general=google&new_tab=1")
	reqPostSettings := httptest.NewRequest("POST", "/settings", form)
	reqPostSettings.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPostSettings := httptest.NewRecorder()
	mux.ServeHTTP(recPostSettings, reqPostSettings)

	if recPostSettings.Code != http.StatusFound {
		t.Errorf("POST /settings returned status %d; want %d", recPostSettings.Code, http.StatusFound)
	}

	// Check cookies in response
	cookies := recPostSettings.Result().Cookies()
	cookieMap := make(map[string]string)
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
	}

	if cookieMap["searxgo_theme"] != "cyberpunk" {
		t.Errorf("expected searxgo_theme=cyberpunk, got %s", cookieMap["searxgo_theme"])
	}
	if cookieMap["searxgo_safesearch"] != "1" {
		t.Errorf("expected searxgo_safesearch=1, got %s", cookieMap["searxgo_safesearch"])
	}
	if cookieMap["searxgo_language"] != "id" {
		t.Errorf("expected searxgo_language=id, got %s", cookieMap["searxgo_language"])
	}
	if cookieMap["searxgo_newtab"] != "true" {
		t.Errorf("expected searxgo_newtab=true, got %s", cookieMap["searxgo_newtab"])
	}

	// Verify GET /settings renders template with saved preferences
	reqGetSettings := httptest.NewRequest("GET", "/settings", nil)
	for _, c := range cookies {
		reqGetSettings.AddCookie(c)
	}
	recGetSettings := httptest.NewRecorder()
	mux.ServeHTTP(recGetSettings, reqGetSettings)

	if recGetSettings.Code != http.StatusOK {
		t.Errorf("GET /settings returned %d, want 200", recGetSettings.Code)
	}
	bodyStr := recGetSettings.Body.String()
	if !strings.Contains(bodyStr, `value="cyberpunk" selected`) {
		t.Errorf("expected template to render cyberpunk selected")
	}
	if !strings.Contains(bodyStr, `value="id" selected`) {
		t.Errorf("expected template to render language id selected")
	}

	// 8. Test POST /settings with default_categories, GET /settings, GET / (ServeIndex), and search fallback
	catForm := strings.NewReader("theme=dark&default_categories=it&default_categories=news")
	reqPostCats := httptest.NewRequest("POST", "/settings", catForm)
	reqPostCats.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPostCats := httptest.NewRecorder()
	mux.ServeHTTP(recPostCats, reqPostCats)

	catCookies := recPostCats.Result().Cookies()
	catCookieMap := make(map[string]string)
	for _, c := range catCookies {
		catCookieMap[c.Name] = c.Value
	}
	if catCookieMap["searxgo_categories"] != "it,news" {
		t.Errorf("expected searxgo_categories=it,news, got %s", catCookieMap["searxgo_categories"])
	}

	// Verify GET /settings renders checked boxes for it and news
	reqGetCatSettings := httptest.NewRequest("GET", "/settings", nil)
	reqGetCatSettings.AddCookie(&http.Cookie{Name: "searxgo_categories", Value: "it,news"})
	recGetCatSettings := httptest.NewRecorder()
	mux.ServeHTTP(recGetCatSettings, reqGetCatSettings)

	if recGetCatSettings.Code != http.StatusOK {
		t.Errorf("GET /settings returned %d, want 200", recGetCatSettings.Code)
	}
	catBodyStr := recGetCatSettings.Body.String()
	if !strings.Contains(catBodyStr, `value="it" checked`) {
		t.Errorf("expected settings template to render it category checked")
	}
	if !strings.Contains(catBodyStr, `value="news" checked`) {
		t.Errorf("expected settings template to render news category checked")
	}

	// Verify GET / (ServeIndex) renders IT category tab as active and checked
	reqGetIndex := httptest.NewRequest("GET", "/", nil)
	reqGetIndex.AddCookie(&http.Cookie{Name: "searxgo_categories", Value: "it,news"})
	recGetIndex := httptest.NewRecorder()
	mux.ServeHTTP(recGetIndex, reqGetIndex)

	if recGetIndex.Code != http.StatusOK {
		t.Errorf("GET / returned %d, want 200", recGetIndex.Code)
	}
	indexBodyStr := recGetIndex.Body.String()
	if !strings.Contains(indexBodyStr, `value="it" checked`) {
		t.Errorf("expected index template to render IT category tab checked")
	}

	// Verify parseSearchRequest falls back to searxgo_categories when category parameter is omitted
	reqSearchFallback := httptest.NewRequest("GET", "/search?q=golang+syntax", nil)
	reqSearchFallback.AddCookie(&http.Cookie{Name: "searxgo_categories", Value: "it,news"})
	parsedReq := h.parseSearchRequest(reqSearchFallback)
	if parsedReq.Category != models.CategoryIT {
		t.Errorf("expected parsed category to be CategoryIT from cookie, got %s", parsedReq.Category)
	}

	// 9. Test POST /settings with full settings suite (hotkeys, method, query_in_title, url_formatting)
	fullForm := strings.NewReader("theme=black&method=POST&hotkeys=off&url_formatting=host&query_in_title=false&doi_rewrite=true")
	reqPostFull := httptest.NewRequest("POST", "/settings", fullForm)
	reqPostFull.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPostFull := httptest.NewRecorder()
	mux.ServeHTTP(recPostFull, reqPostFull)

	fullCookies := recPostFull.Result().Cookies()
	fullCookieMap := make(map[string]string)
	for _, c := range fullCookies {
		fullCookieMap[c.Name] = c.Value
	}
	if fullCookieMap["searxgo_method"] != "POST" {
		t.Errorf("expected searxgo_method=POST, got %s", fullCookieMap["searxgo_method"])
	}
	if fullCookieMap["searxgo_hotkeys"] != "off" {
		t.Errorf("expected searxgo_hotkeys=off, got %s", fullCookieMap["searxgo_hotkeys"])
	}
	if fullCookieMap["searxgo_url_formatting"] != "host" {
		t.Errorf("expected searxgo_url_formatting=host, got %s", fullCookieMap["searxgo_url_formatting"])
	}
	if fullCookieMap["searxgo_query_in_title"] != "false" {
		t.Errorf("expected searxgo_query_in_title=false, got %s", fullCookieMap["searxgo_query_in_title"])
	}

	// Verify GET /settings renders these values selected/checked
	reqGetFullSettings := httptest.NewRequest("GET", "/settings", nil)
	for _, c := range fullCookies {
		reqGetFullSettings.AddCookie(c)
	}
	recGetFullSettings := httptest.NewRecorder()
	mux.ServeHTTP(recGetFullSettings, reqGetFullSettings)
	fullSettingsBody := recGetFullSettings.Body.String()

	if !strings.Contains(fullSettingsBody, `value="off" selected`) {
		t.Errorf("expected hotkeys=off selected in settings HTML")
	}
	if !strings.Contains(fullSettingsBody, `value="POST" selected`) {
		t.Errorf("expected method=POST selected in settings HTML")
	}
}

