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

type googleEndpoint struct {
	Name        string
	Format      string // "single" (translate_a/single) or "t" (clients5 translate_a/t)
	URLTemplate string
	CooldownEnd time.Time
}

var (
	// Dedicated high-performance HTTP client with connection pooling
	transClient = &http.Client{
		Timeout: 5 * time.Second,
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

	// Multi-endpoint rotation pool with circuit breaker
	endpointsMu     sync.Mutex
	endpointIdx     uint64
	googleEndpoints = []*googleEndpoint{
		{
			Name:        "googleapis-gtx",
			Format:      "single",
			URLTemplate: "https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s",
		},
		{
			Name:        "clients5-dict",
			Format:      "t",
			URLTemplate: "https://clients5.google.com/translate_a/t?client=dict-chrome-ex&sl=auto&tl=%s&q=%s",
		},
		{
			Name:        "translate-google-gtx",
			Format:      "single",
			URLTemplate: "https://translate.google.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s",
		},
		{
			Name:        "googleapis-dict",
			Format:      "single",
			URLTemplate: "https://translate.googleapis.com/translate_a/single?client=dict-chrome-ex&sl=auto&tl=%s&dt=t&q=%s",
		},
		{
			Name:        "clients5-gtx",
			Format:      "t",
			URLTemplate: "https://clients5.google.com/translate_a/t?client=gtx&sl=auto&tl=%s&q=%s",
		},
	}

	// Singleflight in-flight request deduplication
	flightMu sync.Mutex
	inFlight = make(map[string]*flightCall)
)

type flightCall struct {
	wg  sync.WaitGroup
	res *TranslationResult
	err error
}

const maxCacheEntries = 10000

func getAvailableGoogleEndpoint() *googleEndpoint {
	endpointsMu.Lock()
	defer endpointsMu.Unlock()

	now := time.Now()
	total := len(googleEndpoints)
	start := int(atomic.AddUint64(&endpointIdx, 1) % uint64(total))

	for i := 0; i < total; i++ {
		ep := googleEndpoints[(start+i)%total]
		if now.After(ep.CooldownEnd) {
			return ep
		}
	}

	// If all are cooling down, return the one whose cooldown expires soonest
	earliest := googleEndpoints[0]
	for _, ep := range googleEndpoints[1:] {
		if ep.CooldownEnd.Before(earliest.CooldownEnd) {
			earliest = ep
		}
	}
	return earliest
}

func markEndpointCooldown(ep *googleEndpoint, d time.Duration) {
	endpointsMu.Lock()
	ep.CooldownEnd = time.Now().Add(d)
	endpointsMu.Unlock()
}

// hasLetters checks if string contains at least one alphabetic character
func hasLetters(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// isNoiseOrLaughter checks if string is pure laugh/filler/noise that doesn't need translation
func isNoiseOrLaughter(s string) bool {
	clean := strings.ToLower(strings.TrimSpace(s))
	if len(clean) <= 1 {
		return true
	}
	if !hasLetters(clean) {
		return true
	}
	// Common chat spam laughters/fillers
	isLaughterChar := true
	for _, r := range clean {
		if r != 'w' && r != 'k' && r != 'h' && r != 'a' && r != 'e' && r != 'u' && r != 'l' && r != 'o' && r != 'x' && r != 'd' {
			isLaughterChar = false
			break
		}
	}
	if isLaughterChar && len(clean) >= 2 {
		if strings.Contains(clean, "wk") || strings.Contains(clean, "ha") ||
			strings.Contains(clean, "he") || strings.Contains(clean, "lol") ||
			strings.Contains(clean, "xd") {
			return true
		}
	}
	return false
}

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

	// Skip non-translatable text (emojis, numbers, laughter spam)
	if isNoiseOrLaughter(clean) {
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

	// In-flight singleflight deduplication
	flightMu.Lock()
	if call, ok := inFlight[cacheKey]; ok {
		flightMu.Unlock()
		call.wg.Wait()
		if call.err != nil {
			return call.res, call.err
		}
		if call.res != nil {
			copyRes := *call.res
			copyRes.Cached = true
			return &copyRes, nil
		}
	}

	call := &flightCall{}
	call.wg.Add(1)
	inFlight[cacheKey] = call
	flightMu.Unlock()

	defer func() {
		flightMu.Lock()
		delete(inFlight, cacheKey)
		flightMu.Unlock()
		call.wg.Done()
	}()

	// 1. Try Google Translate with multi-endpoint pool & circuit breaker
	res, err := translateViaGoogle(ctx, clean, target)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		call.res = res
		return res, nil
	}

	// 2. Fallback to MyMemory API
	res, err = translateViaMyMemory(ctx, clean, target)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		call.res = res
		return res, nil
	}

	// If all methods fail, return original text gracefully
	fallbackRes := &TranslationResult{
		Original:   clean,
		Translated: clean,
		SourceLang: "unknown",
		TargetLang: target,
	}
	call.res = fallbackRes
	return fallbackRes, nil
}

// TranslateBatch translates multiple texts in a single batch to minimize HTTP requests
func TranslateBatch(ctx context.Context, texts []string, targetLang string) ([]*TranslationResult, error) {
	target := strings.ToLower(strings.TrimSpace(targetLang))
	if target == "" {
		target = "id"
	}

	results := make([]*TranslationResult, len(texts))
	uniqueMap := make(map[string][]int) // text -> list of indices

	for i, t := range texts {
		clean := strings.TrimSpace(t)
		if clean == "" || isNoiseOrLaughter(clean) {
			results[i] = &TranslationResult{
				Original:   t,
				Translated: t,
				SourceLang: "auto",
				TargetLang: target,
			}
			continue
		}

		cacheKey := fmt.Sprintf("%s|%s", target, clean)
		if val, ok := transCache.Load(cacheKey); ok {
			cached := val.(*TranslationResult)
			copyRes := *cached
			copyRes.Cached = true
			results[i] = &copyRes
			continue
		}

		uniqueMap[clean] = append(uniqueMap[clean], i)
	}

	if len(uniqueMap) == 0 {
		return results, nil
	}

	// Collect unique texts to translate
	uniqueTexts := make([]string, 0, len(uniqueMap))
	for text := range uniqueMap {
		uniqueTexts = append(uniqueTexts, text)
	}

	// Clean internal newlines so each text is strictly one line
	cleanLines := make([]string, len(uniqueTexts))
	for i, uText := range uniqueTexts {
		cleanLines[i] = strings.ReplaceAll(strings.ReplaceAll(uText, "\r\n", " "), "\n", " ")
	}

	joinedQuery := strings.Join(cleanLines, "\n")
	res, err := translateViaGoogle(ctx, joinedQuery, target)
	if err == nil && res != nil && res.Translated != "" {
		translatedLines := strings.Split(res.Translated, "\n")
		if len(translatedLines) == len(uniqueTexts) {
			for j, uText := range uniqueTexts {
				transLine := strings.TrimSpace(translatedLines[j])
				transRes := &TranslationResult{
					Original:   uText,
					Translated: transLine,
					SourceLang: res.SourceLang,
					TargetLang: target,
				}
				cacheKey := fmt.Sprintf("%s|%s", target, uText)
				cacheTranslation(cacheKey, transRes)

				for _, idx := range uniqueMap[uText] {
					results[idx] = transRes
				}
			}
			return results, nil
		}
	}

	// Fallback: translate individual unique texts if batch alignment failed or errored
	for _, uText := range uniqueTexts {
		singleRes, _ := TranslateText(ctx, uText, target)
		for _, idx := range uniqueMap[uText] {
			results[idx] = singleRes
		}
	}

	return results, nil
}

func cacheTranslation(key string, res *TranslationResult) {
	currentSz := atomic.LoadInt64(&transCacheSz)
	if currentSz >= maxCacheEntries {
		transCache = sync.Map{}
		atomic.StoreInt64(&transCacheSz, 0)
	}

	transCache.Store(key, res)
	atomic.AddInt64(&transCacheSz, 1)
}

func translateViaGoogle(ctx context.Context, text, targetLang string) (*TranslationResult, error) {
	var lastErr error
	totalEndpoints := len(googleEndpoints)

	for attempt := 0; attempt < totalEndpoints; attempt++ {
		ep := getAvailableGoogleEndpoint()
		res, err := callGoogleEndpoint(ctx, ep, text, targetLang)
		if err == nil && res != nil && res.Translated != "" {
			return res, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func callGoogleEndpoint(ctx context.Context, ep *googleEndpoint, text, targetLang string) (*TranslationResult, error) {
	apiURL := fmt.Sprintf(ep.URLTemplate, url.QueryEscape(targetLang), url.QueryEscape(text))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

	resp, err := transClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		markEndpointCooldown(ep, 60*time.Second)
		return nil, fmt.Errorf("google endpoint %s rate limited: %d", ep.Name, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google endpoint %s status %d", ep.Name, resp.StatusCode)
	}

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

	var translated string
	srcLang := "auto"

	if ep.Format == "t" {
		// clients5 format: [["translated text", "detected_lang"]]
		if len(root) > 0 {
			if first, ok := root[0].([]interface{}); ok && len(first) > 0 {
				if transStr, ok := first[0].(string); ok {
					translated = transStr
				}
				if len(first) > 1 {
					if sl, ok := first[1].(string); ok && sl != "" {
						srcLang = sl
					}
				}
			}
		}
	} else {
		// single format: [[["part1", "orig1"], ...], null, "detected_lang"]
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
		translated = strings.Join(translatedParts, "")
		if len(root) > 2 {
			if sl, ok := root[2].(string); ok && sl != "" {
				srcLang = sl
			}
		}
	}

	if strings.TrimSpace(translated) == "" {
		return nil, fmt.Errorf("no translation text extracted from endpoint %s", ep.Name)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := transClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mymemory status %d", resp.StatusCode)
	}

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

// ServeTranslate handles translation HTTP requests supporting single & batch translations
func ServeTranslate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Cache-Control", "public, max-age=180")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var singleText, target string
	var batchTexts []string

	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var body struct {
				Text  string   `json:"text"`
				Texts []string `json:"texts"`
				To    string   `json:"to"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 128*1024)).Decode(&body); err == nil {
				singleText = body.Text
				batchTexts = body.Texts
				target = body.To
			}
		} else {
			_ = r.ParseForm()
			singleText = r.FormValue("text")
			target = r.FormValue("to")
		}
	}

	// Fallback to URL query params
	if singleText == "" && len(batchTexts) == 0 {
		singleText = r.URL.Query().Get("text")
	}
	if target == "" {
		target = r.URL.Query().Get("to")
	}

	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "id"
	}

	// Process batch translation if texts array provided
	if len(batchTexts) > 0 {
		results, err := TranslateBatch(r.Context(), batchTexts, target)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"translations": results,
			"count":        len(results),
			"target_lang":  target,
		})
		return
	}

	singleText = strings.TrimSpace(singleText)
	if singleText == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "text or texts parameter is required"})
		return
	}

	res, err := TranslateText(r.Context(), singleText, target)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}
