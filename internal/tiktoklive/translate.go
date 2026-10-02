package tiktoklive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	transClient = &http.Client{
		Timeout: 4 * time.Second,
	}
	transCache   = sync.Map{}
	transCacheSz int64
)

// TranslateText translates text from auto-detected language to target language
func TranslateText(ctx context.Context, text, targetLang string) (*TranslationResult, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return &TranslationResult{Original: text, Translated: text, SourceLang: "auto", TargetLang: targetLang}, nil
	}

	if targetLang == "" {
		targetLang = "id"
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
			TargetLang: targetLang,
		}, nil
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s|%s", targetLang, clean)
	if val, ok := transCache.Load(cacheKey); ok {
		res := val.(*TranslationResult)
		copyRes := *res
		copyRes.Cached = true
		return &copyRes, nil
	}

	// 1. Try Google Translate client=dict-chrome-ex
	res, err := translateViaGoogle(ctx, clean, targetLang)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		return res, nil
	}

	// 2. Fallback to MyMemory
	res, err = translateViaMyMemory(ctx, clean, targetLang)
	if err == nil && res != nil && res.Translated != "" {
		cacheTranslation(cacheKey, res)
		return res, nil
	}

	// If both fail, return original
	return &TranslationResult{
		Original:   clean,
		Translated: clean,
		SourceLang: "unknown",
		TargetLang: targetLang,
	}, nil
}

func cacheTranslation(key string, res *TranslationResult) {
	if transCacheSz < 3000 {
		transCache.Store(key, res)
		transCacheSz++
	}
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

	body, err := io.ReadAll(resp.Body)
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

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
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

// ServeTranslate handles translation HTTP requests
func ServeTranslate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	text := strings.TrimSpace(r.URL.Query().Get("text"))
	target := strings.TrimSpace(r.URL.Query().Get("to"))
	if target == "" {
		target = "id"
	}

	if text == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "text parameter is required"})
		return
	}

	res, err := TranslateText(r.Context(), text, target)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(res)
}
