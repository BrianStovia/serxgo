package recon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// HeaderAuditItem describes the status of an HTTP security header
type HeaderAuditItem struct {
	Name        string `json:"name"`
	Present     bool   `json:"present"`
	Value       string `json:"value"`
	ScoreImpact int    `json:"score_impact"`
	Description string `json:"description"`
}

// ReconReport is the comprehensive domain security and intelligence report
type ReconReport struct {
	Domain          string                     `json:"domain"`
	SecurityScore   int                        `json:"security_score"`
	SecurityGrade   string                     `json:"security_grade"` // A+, A, B, C, D, F
	ScanTime        string                     `json:"scan_time"`
	ResponseTimeMs  int64                      `json:"response_time_ms"`
	StatusCode      int                        `json:"status_code"`
	FinalURL        string                     `json:"final_url"`
	
	// SSL / TLS
	SSLActive       bool                       `json:"ssl_active"`
	SSLIssuer       string                     `json:"ssl_issuer"`
	SSLSubject      string                     `json:"ssl_subject"`
	SSLValidTo      string                     `json:"ssl_valid_to"`
	SSLDaysLeft     int                        `json:"ssl_days_left"`
	SSLCipher       string                     `json:"ssl_cipher"`
	TLSVersion      string                     `json:"tls_version"`
	
	// HTTP Security Headers
	Headers         map[string]HeaderAuditItem `json:"headers"`
	
	// DNS & Mail Security
	IPv4            []string                   `json:"ipv4"`
	IPv6            []string                   `json:"ipv6"`
	MX              []string                   `json:"mx"`
	NS              []string                   `json:"ns"`
	TXT             []string                   `json:"txt"`
	SPFConfigured   bool                       `json:"spf_configured"`
	SPFRecord       string                     `json:"spf_record,omitempty"`
	DMARCConfigured bool                       `json:"dmarc_configured"`
	DMARCRecord     string                     `json:"dmarc_record,omitempty"`
	
	// Tech Stack & Fingerprinting
	ServerHeader    string                     `json:"server_header,omitempty"`
	PoweredBy       string                     `json:"powered_by,omitempty"`
	DetectedTech    []string                   `json:"detected_tech"`
}

// CleanDomain normalizes user input into a bare domain name
func CleanDomain(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	if idx := strings.Index(raw, "/"); idx != -1 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, ":"); idx != -1 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
}

// AuditDomain executes an all-in-one domain security and intelligence scan
func AuditDomain(ctx context.Context, target string) (*ReconReport, error) {
	domain := CleanDomain(target)
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	start := time.Now()
	report := &ReconReport{
		Domain:       domain,
		ScanTime:     time.Now().UTC().Format(time.RFC3339),
		Headers:      make(map[string]HeaderAuditItem),
		DetectedTech: []string{},
	}

	score := 100

	// 1. SSL/TLS Certificate Inspection
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tlsConn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         domain,
	})
	if err == nil {
		defer tlsConn.Close()
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			report.SSLActive = true
			report.SSLIssuer = cert.Issuer.CommonName
			if report.SSLIssuer == "" && len(cert.Issuer.Organization) > 0 {
				report.SSLIssuer = cert.Issuer.Organization[0]
			}
			report.SSLSubject = cert.Subject.CommonName
			report.SSLValidTo = cert.NotAfter.Format("2006-01-02 15:04:05 UTC")
			report.SSLDaysLeft = int(time.Until(cert.NotAfter).Hours() / 24)
			report.SSLCipher = tls.CipherSuiteName(state.CipherSuite)
			switch state.Version {
			case tls.VersionTLS13:
				report.TLSVersion = "TLS 1.3"
			case tls.VersionTLS12:
				report.TLSVersion = "TLS 1.2"
			default:
				report.TLSVersion = fmt.Sprintf("TLS (%x)", state.Version)
			}
		}
	} else {
		score -= 30 // Massive penalty for lack of HTTPS / SSL
	}

	// 2. HTTP Request & Security Headers Audit
	client := &http.Client{
		Timeout: 4 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	targetURL := "https://" + domain
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGoRecon/1.0")
		resp, err := client.Do(req)
		if err != nil {
			// Fallback to HTTP
			targetURL = "http://" + domain
			req2, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGoRecon/1.0")
			resp, _ = client.Do(req2)
		}

		if resp != nil {
			defer resp.Body.Close()
			report.StatusCode = resp.StatusCode
			if resp.Request != nil && resp.Request.URL != nil {
				report.FinalURL = resp.Request.URL.String()
			}
			report.ResponseTimeMs = time.Since(start).Milliseconds()

			h := resp.Header
			report.ServerHeader = h.Get("Server")
			report.PoweredBy = h.Get("X-Powered-By")

			// Check Security Headers
			checkHeader := func(key, desc string, penalty int) {
				val := h.Get(key)
				item := HeaderAuditItem{
					Name:        key,
					Description: desc,
					Present:     val != "",
					Value:       val,
				}
				if val == "" {
					item.ScoreImpact = -penalty
					score -= penalty
				} else {
					item.ScoreImpact = 0
				}
				report.Headers[key] = item
			}

			checkHeader("Strict-Transport-Security", "Enforces encrypted HTTPS connections and protects against downgrade attacks.", 15)
			checkHeader("Content-Security-Policy", "Restricts resource loading to prevent XSS, clickjacking, and code injection.", 20)
			checkHeader("X-Frame-Options", "Prevents clickjacking attacks by forbidding iframe embedding from foreign origins.", 15)
			checkHeader("X-Content-Type-Options", "Prevents MIME-type sniffing by enforcing declared content types.", 10)
			checkHeader("Referrer-Policy", "Protects user privacy by controlling how much referrer info is leaked.", 10)
			checkHeader("Permissions-Policy", "Restricts browser features like camera, microphone, and geolocation.", 5)

			// Detect Tech Stack
			detectTech(report, resp)
		}
	}

	// 3. DNS & Email Security (SPF & DMARC)
	resolveDNS(ctx, domain, report)

	// Calculate Final Letter Grade
	if score > 100 { score = 100 }
	if score < 0 { score = 0 }
	report.SecurityScore = score

	switch {
	case score >= 90:
		report.SecurityGrade = "A+"
	case score >= 80:
		report.SecurityGrade = "A"
	case score >= 70:
		report.SecurityGrade = "B"
	case score >= 55:
		report.SecurityGrade = "C"
	case score >= 40:
		report.SecurityGrade = "D"
	default:
		report.SecurityGrade = "F"
	}

	return report, nil
}

func detectTech(report *ReconReport, resp *http.Response) {
	h := resp.Header
	techMap := make(map[string]bool)

	add := func(name string) {
		if !techMap[name] {
			techMap[name] = true
			report.DetectedTech = append(report.DetectedTech, name)
		}
	}

	server := strings.ToLower(h.Get("Server"))
	if strings.Contains(server, "cloudflare") { add("Cloudflare CDN & WAF") }
	if strings.Contains(server, "nginx") { add("Nginx") }
	if strings.Contains(server, "apache") { add("Apache HTTP Server") }
	if strings.Contains(server, "caddy") { add("Caddy Server") }
	if strings.Contains(server, "litespeed") { add("LiteSpeed Web Server") }
	if strings.Contains(server, "gunicorn") { add("Gunicorn Python") }

	powered := strings.ToLower(h.Get("X-Powered-By"))
	if strings.Contains(powered, "php") { add("PHP (" + h.Get("X-Powered-By") + ")") }
	if strings.Contains(powered, "express") { add("Node.js / Express") }
	if strings.Contains(powered, "asp.net") { add("ASP.NET") }
	if strings.Contains(powered, "next.js") { add("Next.js React") }

	if h.Get("CF-Ray") != "" { add("Cloudflare Edge Network") }
	if h.Get("X-Varnish") != "" { add("Varnish Cache") }
	if h.Get("X-Cache") != "" { add("Reverse Proxy Cache (" + h.Get("X-Cache") + ")") }
	if strings.Contains(strings.ToLower(h.Get("Set-Cookie")), "wp-") { add("WordPress") }
	if strings.Contains(strings.ToLower(h.Get("Set-Cookie")), "laravel") { add("Laravel Framework") }
}

func resolveDNS(ctx context.Context, domain string, report *ReconReport) {
	resolver := &net.Resolver{}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// A & AAAA
	ips, err := resolver.LookupIP(reqCtx, "ip", domain)
	if err == nil {
		for _, ip := range ips {
			if ip.To4() != nil {
				report.IPv4 = append(report.IPv4, ip.String())
			} else {
				report.IPv6 = append(report.IPv6, ip.String())
			}
		}
	}

	// MX
	mxList, err := resolver.LookupMX(reqCtx, domain)
	if err == nil {
		for _, m := range mxList {
			report.MX = append(report.MX, strings.TrimSuffix(m.Host, "."))
		}
	}

	// NS
	nsList, err := resolver.LookupNS(reqCtx, domain)
	if err == nil {
		for _, n := range nsList {
			report.NS = append(report.NS, strings.TrimSuffix(n.Host, "."))
		}
	}

	// TXT (Check SPF)
	txtList, err := resolver.LookupTXT(reqCtx, domain)
	if err == nil {
		for _, t := range txtList {
			report.TXT = append(report.TXT, t)
			if strings.HasPrefix(strings.ToLower(t), "v=spf1") {
				report.SPFConfigured = true
				report.SPFRecord = t
			}
		}
	}

	// DMARC TXT: _dmarc.<domain>
	dmarcRecords, err := resolver.LookupTXT(reqCtx, "_dmarc."+domain)
	if err == nil {
		for _, d := range dmarcRecords {
			if strings.HasPrefix(strings.ToLower(d), "v=dmarc1") {
				report.DMARCConfigured = true
				report.DMARCRecord = d
				break
			}
		}
	}
}
