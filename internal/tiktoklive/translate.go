package tiktoklive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// TranslationResult represents the translated text output
type TranslationResult struct {
	Original   string `json:"original"`
	Translated string `json:"translated"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
	Cached     bool   `json:"cached,omitempty"`
}

var (
	// High-performance HTTP client with dedicated connection pooling for high-throughput live chat
	transClient = &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   30,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	transCache   sync.Map
	transCacheSz int64
)

const maxCacheEntries = 5000

// TranslateText translates text from auto-detected language to target language
func TranslateText(ctx context.Context, text, targetLang string) (*TranslationResult, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return &TranslationResult{Original: text, Translated: text, SourceLang: "auto", TargetLang: targetLang}, nil
	}

	target := strings.ToLower(strings.TrimSpace(targetLang))
	if target == "" {
		target = "id"
	}

	// If purely emojis, symbols, numbers, skip translation
	hasLetters := false
	for _, r := range clean {
		if unicode.IsLetter(r) {
			hasLetters = true
			break
		}
	}
	if !hasLetters {
		return &TranslationResult{
			Original:   clean,
			Translated: clean,
			SourceLang: "auto",
			TargetLang: target,
		}, nil
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s|%s", target, clean)
	if val, ok := transCache.Load(cacheKey); ok {
		res := val.(*TranslationResult)
		copyRes := *res
		copyRes.Cached = true
		return &copyRes, nil
	}

	// 1. Try Google Translate client=dict-chrome-ex (Fast & Accurate)
	res, err := translateViaGoogle(ctx, clean, target)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		return res, nil
	}

	// 2. Fallback to MyMemory API
	res, err = translateViaMyMemory(ctx, clean, target)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		return res, nil
	}

	// If both fail, return original text gracefully
	return &TranslationResult{
		Original:   clean,
		Translated: clean,
		SourceLang: "unknown",
		TargetLang: target,
	}, nil
}

func cacheTranslation(key string, res *TranslationResult) {
	currentSz := atomic.LoadInt64(&transCacheSz)
	if currentSz >= maxCacheEntries {
		// Auto-eviction: clear old cache once max capacity is reached to prevent memory leak
		transCache = sync.Map{}
		atomic.StoreInt64(&transCacheSz, 0)
	}

	transCache.Store(key, res)
	atomic.AddInt64(&transCacheSz, 1)
}

func translateViaGoogle(ctx context.Context, text, targetLang string) (*TranslationResult, error) {
	apiURL := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=dict-chrome-ex&sl=auto&tl=%s&dt=t&q=%s",
		url.QueryEscape(targetLang), url.QueryEscape(text))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	resp, err := transClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google translate status %d", resp.StatusCode)
	}

	// Read limited body (max 512KB) to protect memory
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	var root []interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}

	if len(root) == 0 {
		return nil, fmt.Errorf("empty google translate response")
	}

	var translatedParts []string
	if sentences, ok := root[0].([]interface{}); ok {
		for _, s := range sentences {
			if sArr, ok := s.([]interface{}); ok && len(sArr) > 0 {
				if part, ok := sArr[0].(string); ok {
					translatedParts = append(translatedParts, part)
				}
			}
		}
	}

	translated := strings.Join(translatedParts, "")
	if translated == "" {
		return nil, fmt.Errorf("no translation text extracted")
	}

	srcLang := "auto"
	if len(root) > 2 {
		if sl, ok := root[2].(string); ok && sl != "" {
			srcLang = sl
		}
	}

	return &TranslationResult{
		Original:   text,
		Translated: translated,
		SourceLang: srcLang,
		TargetLang: targetLang,
	}, nil
}

func translateViaMyMemory(ctx context.Context, text, targetLang string) (*TranslationResult, error) {
	apiURL := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=autodetect|%s",
		url.QueryEscape(text), url.QueryEscape(targetLang))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	resp, err := transClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		ResponseData struct {
			TranslatedText   string `json:"translatedText"`
			DetectedLanguage string `json:"detectedLanguage"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&data); err != nil {
		return nil, err
	}

	if data.ResponseData.TranslatedText == "" {
		return nil, fmt.Errorf("empty mymemory translation")
	}

	return &TranslationResult{
		Original:   text,
		Translated: data.ResponseData.TranslatedText,
		SourceLang: data.ResponseData.DetectedLanguage,
		TargetLang: targetLang,
	}, nil
}

// ServeTranslate handles translation HTTP requests supporting both GET and POST
func ServeTranslate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Cache-Control", "public, max-age=120")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var text, target string

	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var body struct {
				Text string `json:"text"`
				To   string `json:"to"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&body); err == nil {
				text = body.Text
				target = body.To
			}
		} else {
			_ = r.ParseForm()
			text = r.FormValue("text")
			target = r.FormValue("to")
		}
	}

	// Fallback to URL query parameters if not provided in POST body
	if text == "" {
		text = r.URL.Query().Get("text")
	}
	if target == "" {
		target = r.URL.Query().Get("to")
	}

	text = strings.TrimSpace(text)
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "id"
	}

	if text == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "text parameter is required"})
		return
	}

	res, err := TranslateText(r.Context(), text, target)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}
