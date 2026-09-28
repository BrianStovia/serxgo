package apitester

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RequestPayload represents the API request from client
type RequestPayload struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	FollowRedirects bool              `json:"follow_redirects"`
}

// ResponsePayload holds the response details and generated code snippets
type ResponsePayload struct {
	StatusCode   int               `json:"status_code"`
	StatusText   string            `json:"status_text"`
	DurationMs   int64             `json:"duration_ms"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body"`
	SizeBytes    int64             `json:"size_bytes"`
	ContentType  string            `json:"content_type"`
	CodeSnippets map[string]string `json:"code_snippets"`
	Error        string            `json:"error,omitempty"`
}

// ExecuteRequest runs the HTTP request safely and returns response & code snippets
func ExecuteRequest(ctx context.Context, payload RequestPayload) *ResponsePayload {
	start := time.Now()
	res := &ResponsePayload{
		Headers:      make(map[string]string),
		CodeSnippets: make(map[string]string),
	}

	rawURL := strings.TrimSpace(payload.URL)
	if rawURL == "" {
		res.Error = "URL is required"
		return res
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
		payload.URL = rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		res.Error = fmt.Sprintf("Invalid URL: %v", err)
		return res
	}

	// SSRF check for private / internal addresses
	host := parsedURL.Hostname()
	if isPrivateHost(host) {
		res.Error = "Connecting to localhost or private loopback addresses is restricted for security."
		return res
	}

	method := strings.ToUpper(strings.TrimSpace(payload.Method))
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if payload.Body != "" && method != "GET" && method != "HEAD" {
		bodyReader = bytes.NewBufferString(payload.Body)
	}

	// Setup custom client with timeout
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	if !payload.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		res.Error = fmt.Sprintf("Failed to construct request: %v", err)
		return res
	}

	// Default user agent if not provided
	hasUA := false
	for k, v := range payload.Headers {
		if strings.EqualFold(k, "user-agent") {
			hasUA = true
		}
		req.Header.Set(k, v)
	}
	if !hasUA {
		req.Header.Set("User-Agent", "SearXGo-APITester/1.0")
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Error = fmt.Sprintf("Request failed: %v", err)
		res.DurationMs = time.Since(start).Milliseconds()
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.StatusText = resp.Status
	res.DurationMs = time.Since(start).Milliseconds()

	// Capture response headers
	for k, vals := range resp.Header {
		res.Headers[k] = strings.Join(vals, ", ")
	}
	res.ContentType = resp.Header.Get("Content-Type")

	// Limit body read to 1MB max
	limitReader := io.LimitReader(resp.Body, 1024*1024)
	bodyBytes, _ := io.ReadAll(limitReader)
	res.SizeBytes = int64(len(bodyBytes))
	res.Body = string(bodyBytes)

	// Generate ready-to-use code snippets
	res.CodeSnippets["curl"] = GenerateCurl(payload)
	res.CodeSnippets["go"] = GenerateGo(payload)
	res.CodeSnippets["python"] = GeneratePython(payload)
	res.CodeSnippets["javascript"] = GenerateJS(payload)

	return res
}

// GenerateCurl produces equivalent cURL command
func GenerateCurl(p RequestPayload) string {
	var sb strings.Builder
	method := strings.ToUpper(p.Method)
	if method == "" {
		method = "GET"
	}

	sb.WriteString(fmt.Sprintf("curl -X %s '%s'", method, p.URL))
	for k, v := range p.Headers {
		sb.WriteString(fmt.Sprintf(" \\\n  -H '%s: %s'", k, v))
	}
	if p.Body != "" && method != "GET" && method != "HEAD" {
		escaped := strings.ReplaceAll(p.Body, "'", "'\\''")
		sb.WriteString(fmt.Sprintf(" \\\n  -d '%s'", escaped))
	}
	return sb.String()
}

// GenerateGo produces equivalent Go net/http code snippet
func GenerateGo(p RequestPayload) string {
	method := strings.ToUpper(p.Method)
	if method == "" {
		method = "GET"
	}

	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n")
	if p.Body != "" && method != "GET" {
		sb.WriteString("\t\"strings\"\n")
	}
	sb.WriteString(")\n\nfunc main() {\n")

	if p.Body != "" && method != "GET" {
		escaped := strings.ReplaceAll(p.Body, "`", "`+\"`\"+`")
		sb.WriteString(fmt.Sprintf("\tbody := strings.NewReader(`%s`)\n", escaped))
		sb.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(\"%s\", \"%s\", body)\n", method, p.URL))
	} else {
		sb.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(\"%s\", \"%s\", nil)\n", method, p.URL))
	}
	sb.WriteString("\tif err != nil {\n\t\tpanic(err)\n\t}\n")

	for k, v := range p.Headers {
		sb.WriteString(fmt.Sprintf("\treq.Header.Set(\"%s\", \"%s\")\n", k, v))
	}

	sb.WriteString("\tresp, err := http.DefaultClient.Do(req)\n")
	sb.WriteString("\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	sb.WriteString("\tdefer resp.Body.Close()\n")
	sb.WriteString("\tdata, _ := io.ReadAll(resp.Body)\n")
	sb.WriteString("\tfmt.Println(string(data))\n}\n")

	return sb.String()
}

// GeneratePython produces Python requests code
func GeneratePython(p RequestPayload) string {
	method := strings.ToLower(p.Method)
	if method == "" {
		method = "get"
	}

	var sb strings.Builder
	sb.WriteString("import requests\n\n")
	sb.WriteString(fmt.Sprintf("url = \"%s\"\n", p.URL))

	if len(p.Headers) > 0 {
		sb.WriteString("headers = {\n")
		for k, v := range p.Headers {
			sb.WriteString(fmt.Sprintf("    \"%s\": \"%s\",\n", k, v))
		}
		sb.WriteString("}\n")
	} else {
		sb.WriteString("headers = {}\n")
	}

	if p.Body != "" && method != "get" && method != "head" {
		escaped := strings.ReplaceAll(p.Body, "\"", "\\\"")
		sb.WriteString(fmt.Sprintf("payload = \"\"\"%s\"\"\"\n", escaped))
		sb.WriteString(fmt.Sprintf("response = requests.%s(url, headers=headers, data=payload)\n", method))
	} else {
		sb.WriteString(fmt.Sprintf("response = requests.%s(url, headers=headers)\n", method))
	}

	sb.WriteString("print(response.status_code)\n")
	sb.WriteString("print(response.text)\n")

	return sb.String()
}

// GenerateJS produces modern JavaScript fetch code
func GenerateJS(p RequestPayload) string {
	method := strings.ToUpper(p.Method)
	if method == "" {
		method = "GET"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("fetch('%s', {\n", p.URL))
	sb.WriteString(fmt.Sprintf("  method: '%s',\n", method))

	if len(p.Headers) > 0 {
		sb.WriteString("  headers: {\n")
		for k, v := range p.Headers {
			sb.WriteString(fmt.Sprintf("    '%s': '%s',\n", k, v))
		}
		sb.WriteString("  },\n")
	}

	if p.Body != "" && method != "GET" && method != "HEAD" {
		escaped := strings.ReplaceAll(p.Body, "`", "\\`")
		sb.WriteString(fmt.Sprintf("  body: `%s`,\n", escaped))
	}

	sb.WriteString("})\n")
	sb.WriteString(".then(res => res.text())\n")
	sb.WriteString(".then(data => console.log(data))\n")
	sb.WriteString(".catch(err => console.error(err));\n")

	return sb.String()
}

func isPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
	}
	return false
}
