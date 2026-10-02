package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/apitester"
	"searxgo/internal/bypass"
	"searxgo/internal/currency"
	"searxgo/internal/models"
	"searxgo/internal/qr"
	"searxgo/internal/reader"
	"searxgo/internal/stats"
	"searxgo/internal/tiktoklive"
)

// ==============================================================================
// 📊 Telemetry & System Statistics Handlers
// ==============================================================================

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

// ==============================================================================
// 🐕 Watchdog RSS / JSON Feed Handlers
// ==============================================================================

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

	h.ServeRSS(w, r, req)
}

// ==============================================================================
// 🖼️ Reverse Image Search & File Upload Handlers
// ==============================================================================

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
					if data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(rawB64))); err == nil {
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
	} else {
		targetImageURL = strings.TrimSpace(r.URL.Query().Get("url"))
		if targetImageURL == "" {
			targetImageURL = strings.TrimSpace(r.URL.Query().Get("image_url"))
		}
	}

	if targetImageURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ReverseImageResponse{
			Success: false,
			Error:   "Please provide an image file, base64 payload, or image URL",
		})
		return
	}

	resp := ReverseImageResponse{
		Success:    true,
		ImageURL:   targetImageURL,
		Filename:   filename,
		Size:       fileSize,
		Dimensions: dimensions,
		MimeType:   mimeType,
		Engines:    h.buildReverseSearchEngines(targetImageURL),
	}

	json.NewEncoder(w).Encode(resp)
}

// ==============================================================================
// 🕸️ Knowledge Graph & Entity Extraction Handlers
// ==============================================================================

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
// 📖 Reader Mode Handlers
// ==============================================================================

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
// 🔴 TikTok Live Direct Stream Viewer & Proxy Handlers
// ==============================================================================

func (h *Handler) ServeTikTokLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	if user == "" {
		user = strings.TrimSpace(r.URL.Query().Get("u"))
	}
	data := map[string]interface{}{
		"User": user,
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "tiktoklive.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (h *Handler) ServeAPITikTokLiveInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	user := strings.TrimSpace(r.URL.Query().Get("user"))
	if user == "" {
		user = strings.TrimSpace(r.URL.Query().Get("u"))
	}
	if user == "" {
		user = strings.TrimSpace(r.URL.Query().Get("url"))
	}
	if user == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user or url parameter is required"})
		return
	}

	info, err := h.tiktokLiveService.Extract(r.Context(), user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(info)
}

func (h *Handler) ServeAPITikTokLiveProxy(w http.ResponseWriter, r *http.Request) {
	tiktoklive.ServeProxy(w, r)
}

func (h *Handler) ServeAPITikTokLiveChat(w http.ResponseWriter, r *http.Request) {
	tiktoklive.ServeChatSSE(w, r)
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
	if r.URL.Query().Get("refresh") == "1" {
		h.newsHubService.ClearCache(topic, region)
	}

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

// ==============================================================================
// 🛠️ Interactive API Tester Playground Handlers
// ==============================================================================

func (h *Handler) ServeAPITester(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "apitester.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func (h *Handler) ServeAPIExecuteRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var payload apitester.RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload: " + err.Error()})
		return
	}

	res := apitester.ExecuteRequest(r.Context(), payload)
	json.NewEncoder(w).Encode(res)
}
