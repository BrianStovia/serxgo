package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/cloudrecon"
	"searxgo/internal/cve"
	"searxgo/internal/dorks"
	"searxgo/internal/instant"
	"searxgo/internal/ipintel"
	"searxgo/internal/recon"
	"searxgo/internal/scrub"
	"searxgo/internal/threat"
)

// ==============================================================================
// 🎯 Google Dorking & Reconnaissance Handlers
// ==============================================================================

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

// ==============================================================================
// 🧼 EXIF Scrubbing & Privacy Cleaner Handlers
// ==============================================================================

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

// ==============================================================================
// 🔬 Domain Recon & Security Auditor Handlers
// ==============================================================================

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

// ==============================================================================
// 🕵️ OSINT Sherlock Username Recon Handlers
// ==============================================================================

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

// ==============================================================================
// 🛡️ URL Threat Intelligence & Sandbox Handlers
// ==============================================================================

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
// 🛡️ Live CVE & Zero-Day Feed Handlers
// ==============================================================================

// ServeCVE renders the Live CVE & Zero-Day feed workspace
func (h *Handler) ServeCVE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "cve.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPICVE returns filtered CVE items in JSON format
func (h *Handler) ServeAPICVE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	query := r.URL.Query().Get("q")
	severity := r.URL.Query().Get("severity")
	kevOnly := r.URL.Query().Get("kev") == "true" || r.URL.Query().Get("kev") == "1"

	svc := cve.GetService()
	res := svc.GetCVEs(r.Context(), query, severity, kevOnly)
	json.NewEncoder(w).Encode(res)
}

// ==============================================================================
// 📍 IP Intelligence & ASN Visualizer Handlers
// ==============================================================================

// ServeIPIntel renders the IP Intelligence & ASN Visualizer
func (h *Handler) ServeIPIntel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "ipintel.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPIIPIntel handles IP & ASN intelligence lookups
func (h *Handler) ServeAPIIPIntel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("ip"))
	if target == "" {
		target = strings.TrimSpace(r.URL.Query().Get("target"))
	}
	if target == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && host != "" {
			target = host
		} else {
			target = r.RemoteAddr
		}
	}

	report := ipintel.Investigate(r.Context(), target)
	json.NewEncoder(w).Encode(report)
}

// ==============================================================================
// ☁️ Cloud Bucket & Public Storage Recon Handlers
// ==============================================================================

// ServeCloudRecon renders the cloud storage and bucket exposure workspace
func (h *Handler) ServeCloudRecon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "cloudrecon.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// ServeAPICloudRecon audits public cloud bucket exposure for a target name
func (h *Handler) ServeAPICloudRecon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = strings.TrimSpace(r.URL.Query().Get("name"))
	}
	if target == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Parameter 'target' is required"})
		return
	}

	report := cloudrecon.ScanBuckets(r.Context(), target)
	json.NewEncoder(w).Encode(report)
}
