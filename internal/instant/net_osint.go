package instant

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"searxgo/internal/models"
)

// CheckNetworkOSINTQuery parses and executes instant network & security intelligence queries
func CheckNetworkOSINTQuery(ctx context.Context, query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. DNS Lookup: "dns google.com", "dns: cloudflare.com", "dig example.com", "nslookup target.com"
	dnsPrefixes := []string{"dns:", "dns ", "dig ", "nslookup ", "records "}
	for _, p := range dnsPrefixes {
		if strings.HasPrefix(qLower, p) {
			domain := cleanDomainInput(q[len(p):])
			if domain != "" {
				return checkDNSRecords(ctx, domain)
			}
		}
	}

	// 2. SSL/TLS Certificate: "ssl google.com", "ssl: github.com", "cert tesla.com", "tls example.com"
	sslPrefixes := []string{"ssl:", "ssl ", "cert:", "cert ", "tls:", "tls ", "certificate "}
	for _, p := range sslPrefixes {
		if strings.HasPrefix(qLower, p) {
			domain := cleanDomainInput(q[len(p):])
			if domain != "" {
				return checkSSLCertificate(ctx, domain)
			}
		}
	}

	// 3. HTTP Security Headers: "headers google.com", "headers: github.com", "security headers example.com"
	headerPrefixes := []string{"headers:", "headers ", "security headers ", "http headers "}
	for _, p := range headerPrefixes {
		if strings.HasPrefix(qLower, p) {
			domain := cleanDomainInput(q[len(p):])
			if domain != "" {
				return checkSecurityHeaders(ctx, domain)
			}
		}
	}

	// 4. WHOIS / RDAP / ASN: "whois google.com", "whois: example.com", "rdap target.org", "asn 1.1.1.1"
	whoisPrefixes := []string{"whois:", "whois ", "rdap:", "rdap ", "asn:", "asn "}
	for _, p := range whoisPrefixes {
		if strings.HasPrefix(qLower, p) {
			target := cleanDomainInput(q[len(p):])
			if target != "" {
				return checkWhoisRDAP(ctx, target)
			}
		}
	}

	// 5. Subdomain Finder via Certificate Transparency: "subdomains tesla.com", "subdomains: github.com", "subdomain uber.com"
	subdomainPrefixes := []string{"subdomains:", "subdomains ", "subdomain:", "subdomain ", "find subdomains "}
	for _, p := range subdomainPrefixes {
		if strings.HasPrefix(qLower, p) {
			domain := cleanDomainInput(q[len(p):])
			if domain != "" {
				return checkSubdomains(ctx, domain)
			}
		}
	}

	// 6. CVE / Vulnerability lookup: "cve: cve-2021-44228", "cve log4j", "cve-2024-3094"
	if strings.HasPrefix(qLower, "cve:") || strings.HasPrefix(qLower, "cve ") || (strings.HasPrefix(qLower, "cve-") && len(qLower) >= 8) {
		target := strings.TrimPrefix(qLower, "cve:")
		target = strings.TrimPrefix(target, "cve ")
		target = strings.TrimSpace(target)
		if target != "" {
			return checkCVELookup(ctx, target)
		}
	}

	return nil
}

func cleanDomainInput(raw string) string {
	raw = strings.TrimSpace(raw)
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

// 1. DNS Records Resolver
func checkDNSRecords(ctx context.Context, domain string) *models.InstantAnswer {
	resolver := &net.Resolver{PreferGo: true}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	attrs := make(map[string]string)

	// A & AAAA
	ips, err := resolver.LookupIP(reqCtx, "ip", domain)
	if err == nil && len(ips) > 0 {
		var aList, aaaaList []string
		for _, ip := range ips {
			if ip.To4() != nil {
				aList = append(aList, ip.String())
			} else {
				aaaaList = append(aaaaList, ip.String())
			}
		}
		if len(aList) > 0 {
			attrs["A (IPv4)"] = strings.Join(aList, ", ")
		}
		if len(aaaaList) > 0 {
			attrs["AAAA (IPv6)"] = strings.Join(aaaaList, ", ")
		}
	}

	// MX
	mxRecords, err := resolver.LookupMX(reqCtx, domain)
	if err == nil && len(mxRecords) > 0 {
		var mxList []string
		for _, mx := range mxRecords {
			mxList = append(mxList, fmt.Sprintf("%s (pref %d)", strings.TrimSuffix(mx.Host, "."), mx.Pref))
		}
		attrs["MX (Mail)"] = strings.Join(mxList, " | ")
	}

	// NS
	nsRecords, err := resolver.LookupNS(reqCtx, domain)
	if err == nil && len(nsRecords) > 0 {
		var nsList []string
		for _, ns := range nsRecords {
			nsList = append(nsList, strings.TrimSuffix(ns.Host, "."))
		}
		attrs["NS (Nameservers)"] = strings.Join(nsList, ", ")
	}

	// TXT
	txtRecords, err := resolver.LookupTXT(reqCtx, domain)
	if err == nil && len(txtRecords) > 0 {
		var cleanTxt []string
		for _, t := range txtRecords {
			if len(t) > 60 {
				t = t[:57] + "..."
			}
			cleanTxt = append(cleanTxt, t)
		}
		attrs["TXT Records"] = strings.Join(cleanTxt, " | ")
	}

	// CNAME
	cname, err := resolver.LookupCNAME(reqCtx, domain)
	if err == nil && cname != "" && !strings.EqualFold(strings.TrimSuffix(cname, "."), domain) {
		attrs["CNAME"] = strings.TrimSuffix(cname, ".")
	}

	if len(attrs) == 0 {
		return nil
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🌐 DNS Records: %s", domain),
		Value:       fmt.Sprintf("%d record types resolved", len(attrs)),
		Description: fmt.Sprintf("Live authoritative DNS query for %s resolving A, AAAA, MX, NS, TXT, and CNAME records.", domain),
		URL:         fmt.Sprintf("https://mxtoolbox.com/SuperTool.aspx?action=dns%%3a%s", url.QueryEscape(domain)),
		Attributes:  attrs,
	}
}

// 2. SSL/TLS Certificate Handshake Inspector
func checkSSLCertificate(ctx context.Context, domain string) *models.InstantAnswer {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tlsConfig := &tls.Config{
		ServerName: domain,
		InsecureSkipVerify: false,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", tlsConfig)
	if err != nil {
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("🔒 SSL/TLS Certificate: %s", domain),
			Value:       "SSL Connection Failed or Untrusted",
			Description: fmt.Sprintf("Handshake error: %v", err),
			URL:         fmt.Sprintf("https://www.ssllabs.com/ssltest/analyze.html?d=%s", url.QueryEscape(domain)),
		}
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil
	}

	cert := state.PeerCertificates[0]
	now := time.Now()
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)

	status := "Valid"
	if daysLeft < 0 {
		status = "EXPIRED"
	} else if daysLeft < 15 {
		status = fmt.Sprintf("Expiring soon (%d days left)", daysLeft)
	} else {
		status = fmt.Sprintf("Valid (%d days remaining)", daysLeft)
	}

	attrs := map[string]string{
		"Subject (Common Name)": cert.Subject.CommonName,
		"Issuer (Authority)":    cert.Issuer.CommonName,
		"Valid From":            cert.NotBefore.Format("2006-01-02"),
		"Valid Until":           cert.NotAfter.Format("2006-01-02"),
		"Status":                status,
		"TLS Version":           tlsVersionString(state.Version),
		"Cipher Suite":          tls.CipherSuiteName(state.CipherSuite),
	}

	if len(cert.DNSNames) > 0 {
		sanDisplay := strings.Join(cert.DNSNames, ", ")
		if len(sanDisplay) > 80 {
			sanDisplay = sanDisplay[:77] + "..."
		}
		attrs["SAN Domains"] = sanDisplay
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🔒 SSL/TLS Certificate: %s", domain),
		Value:       status,
		Description: fmt.Sprintf("Certificate issued by %s, valid until %s.", cert.Issuer.CommonName, cert.NotAfter.Format("2006-01-02")),
		URL:         fmt.Sprintf("https://www.ssllabs.com/ssltest/analyze.html?d=%s", url.QueryEscape(domain)),
		Attributes:  attrs,
	}
}

func tlsVersionString(ver uint16) string {
	switch ver {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("TLS 0x%04x", ver)
	}
}

// 3. HTTP Security Headers Inspector
func checkSecurityHeaders(ctx context.Context, domain string) *models.InstantAnswer {
	targetURL := "https://" + domain
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "HEAD", targetURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Fallback to GET if HEAD rejected
		req.Method = "GET"
		resp, err = client.Do(req)
		if err != nil {
			return nil
		}
	}
	defer resp.Body.Close()

	attrs := make(map[string]string)
	h := resp.Header

	checkHeader := func(name string) {
		val := h.Get(name)
		if val != "" {
			if len(val) > 75 {
				val = val[:72] + "..."
			}
			attrs[name] = val
		} else {
			attrs[name] = "❌ Missing"
		}
	}

	checkHeader("Strict-Transport-Security")
	checkHeader("Content-Security-Policy")
	checkHeader("X-Frame-Options")
	checkHeader("X-Content-Type-Options")
	checkHeader("Referrer-Policy")
	checkHeader("Cross-Origin-Opener-Policy")
	if server := h.Get("Server"); server != "" {
		attrs["Server Banner"] = server
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🛡️ HTTP Security Headers: %s", domain),
		Value:       fmt.Sprintf("HTTP %s", resp.Status),
		Description: fmt.Sprintf("Security headers audit for %s analyzing transport security, framing policies, and content protections.", domain),
		URL:         fmt.Sprintf("https://securityheaders.com/?q=%s", url.QueryEscape(targetURL)),
		Attributes:  attrs,
	}
}

// 4. WHOIS / RDAP Registry Query
func checkWhoisRDAP(ctx context.Context, target string) *models.InstantAnswer {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("https://rdap.org/domain/%s", url.PathEscape(target))
	req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "SearXGo-RDAP-Inspector/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var rdapData struct {
		LdhName string `json:"ldhName"`
		Handle  string `json:"handle"`
		Status  []string `json:"status"`
		Events  []struct {
			EventAction string `json:"eventAction"`
			EventDate   string `json:"eventDate"`
		} `json:"events"`
		Entities []struct {
			Roles []string `json:"roles"`
			VcardArray []interface{} `json:"vcardArray"`
		} `json:"entities"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rdapData); err != nil {
		return nil
	}

	attrs := make(map[string]string)
	if rdapData.LdhName != "" {
		attrs["Domain Name"] = rdapData.LdhName
	}

	for _, ev := range rdapData.Events {
		dateStr := ev.EventDate
		if t, err := time.Parse(time.RFC3339, ev.EventDate); err == nil {
			dateStr = t.Format("2006-01-02")
		}
		switch ev.EventAction {
		case "registration":
			attrs["Registered On"] = dateStr
		case "expiration":
			attrs["Expires On"] = dateStr
		case "last changed":
			attrs["Last Updated"] = dateStr
		}
	}

	if len(rdapData.Status) > 0 {
		attrs["Domain Status"] = strings.Join(rdapData.Status[:min(2, len(rdapData.Status))], ", ")
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("📋 WHOIS / RDAP Registry: %s", target),
		Value:       "Authoritative ICANN Registration Record",
		Description: fmt.Sprintf("Registry registration details, creation and expiration timeline for %s.", target),
		URL:         fmt.Sprintf("https://lookup.icann.org/en/lookup?q=%s", url.QueryEscape(target)),
		Attributes:  attrs,
	}
}

// 5. Subdomain Finder via Certificate Transparency (crt.sh)
func checkSubdomains(ctx context.Context, domain string) *models.InstantAnswer {
	reqCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()

	apiURL := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", url.QueryEscape(domain))
	req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 3500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var entries []struct {
		NameValue string `json:"name_value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil || len(entries) == 0 {
		return nil
	}

	subdomainMap := make(map[string]bool)
	domainLower := strings.ToLower(domain)

	for _, entry := range entries {
		lines := strings.Split(entry.NameValue, "\n")
		for _, l := range lines {
			l = strings.ToLower(strings.TrimSpace(l))
			l = strings.TrimPrefix(l, "*.")
			if strings.HasSuffix(l, "."+domainLower) && l != domainLower {
				subdomainMap[l] = true
			}
		}
	}

	if len(subdomainMap) == 0 {
		return nil
	}

	var subList []string
	for sub := range subdomainMap {
		subList = append(subList, sub)
	}
	sort.Strings(subList)

	sampleCount := min(12, len(subList))
	topSubdomains := strings.Join(subList[:sampleCount], ", ")
	if len(subList) > sampleCount {
		topSubdomains += fmt.Sprintf(" ... and %d more", len(subList)-sampleCount)
	}

	attrs := map[string]string{
		"Total Discovered": fmt.Sprintf("%d unique subdomains", len(subList)),
		"Sample Subdomains": topSubdomains,
		"Data Source":      "Certificate Transparency Logs (crt.sh)",
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🔍 Subdomain Transparency: %s", domain),
		Value:       fmt.Sprintf("%d Subdomains Found", len(subList)),
		Description: fmt.Sprintf("Public SSL/TLS certificate transparency records revealing active subdomains for %s.", domain),
		URL:         fmt.Sprintf("https://crt.sh/?q=%%25.%s", url.QueryEscape(domain)),
		Attributes:  attrs,
	}
}

// 6. CVE & Vulnerability Lookup (OSV.dev API)
func checkCVELookup(ctx context.Context, target string) *models.InstantAnswer {
	cveUpper := strings.ToUpper(strings.TrimSpace(target))
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("https://api.osv.dev/v1/vulns/%s", url.PathEscape(cveUpper))
	req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "SearXGo-CVE-Inspector/1.0")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		// If direct OSV ID lookup failed, return an informative OSINT research card
		return &models.InstantAnswer{
			Type:        "infobox",
			Title:       fmt.Sprintf("🛡️ CVE Security Vulnerability: %s", cveUpper),
			Value:       "Common Vulnerabilities and Exposures",
			Description: fmt.Sprintf("Query NIST NVD, MITRE CVE, and OSV vulnerability databases for '%s'.", cveUpper),
			URL:         fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", url.PathEscape(cveUpper)),
			Attributes: map[string]string{
				"NIST NVD":  fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", url.PathEscape(cveUpper)),
				"MITRE CVE": fmt.Sprintf("https://cve.mitre.org/cgi-bin/cvename.cgi?name=%s", url.PathEscape(cveUpper)),
				"OSV.dev":   fmt.Sprintf("https://osv.dev/vulnerability/%s", url.PathEscape(cveUpper)),
			},
		}
	}
	defer resp.Body.Close()

	var cveData struct {
		ID       string `json:"id"`
		Summary  string `json:"summary"`
		Details  string `json:"details"`
		Modified string `json:"modified"`
		Published string `json:"published"`
		DatabaseSpecific struct {
			Severity string `json:"severity"`
		} `json:"database_specific"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cveData); err != nil {
		return nil
	}

	attrs := map[string]string{
		"Vulnerability ID": cveData.ID,
	}
	if cveData.Published != "" {
		attrs["Published Date"] = cveData.Published[:min(10, len(cveData.Published))]
	}
	if cveData.DatabaseSpecific.Severity != "" {
		attrs["Severity"] = cveData.DatabaseSpecific.Severity
	}

	desc := cveData.Summary
	if desc == "" {
		desc = cveData.Details
	}
	if len(desc) > 200 {
		desc = desc[:197] + "..."
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🛡️ Vulnerability Report: %s", cveData.ID),
		Value:       desc,
		Description: fmt.Sprintf("Published %s. Verified in Open Source Vulnerabilities (OSV) database.", cveData.Published),
		URL:         fmt.Sprintf("https://osv.dev/vulnerability/%s", url.PathEscape(cveData.ID)),
		Attributes:  attrs,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
