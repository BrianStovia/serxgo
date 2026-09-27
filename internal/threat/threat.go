package threat

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ThreatLevel represents severity of a detected threat signal
type ThreatLevel string

const (
	ThreatSafe     ThreatLevel = "safe"
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// ThreatSignal represents a single detected threat indicator
type ThreatSignal struct {
	Code        string      `json:"code"`
	Category    string      `json:"category"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Level       ThreatLevel `json:"level"`
}

// RedirectHop describes one redirect in the chain
type RedirectHop struct {
	Step       int    `json:"step"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	IsHTTPS    bool   `json:"is_https"`
}

// ThreatReport is the full URL threat intelligence output
type ThreatReport struct {
	InputURL      string         `json:"input_url"`
	FinalURL      string         `json:"final_url"`
	FinalDomain   string         `json:"final_domain"`
	FinalStatusCode int          `json:"final_status_code"`
	ScanDuration  string         `json:"scan_duration"`
	ThreatLevel   ThreatLevel    `json:"threat_level"`
	ThreatScore   int            `json:"threat_score"` // 0 = clean, 100 = critical
	Signals       []ThreatSignal `json:"signals"`
	RedirectChain []RedirectHop  `json:"redirect_chain"`
	IsShortened   bool           `json:"is_shortened"`
	HasSSL        bool           `json:"has_ssl"`
	SSLIssuer     string         `json:"ssl_issuer,omitempty"`
	SSLValidTo    string         `json:"ssl_valid_to,omitempty"`
	SSLDaysLeft   int            `json:"ssl_days_left,omitempty"`
	IPv4          []string       `json:"ipv4,omitempty"`
	ServerHeader  string         `json:"server_header,omitempty"`
	ContentType   string         `json:"content_type,omitempty"`
	Summary       string         `json:"summary"`
}

// Known URL shortener domains
var urlShorteners = map[string]bool{
	"bit.ly": true, "t.co": true, "tinyurl.com": true, "ow.ly": true,
	"is.gd": true, "buff.ly": true, "adf.ly": true, "tr.im": true,
	"x.co": true, "short.link": true, "cutt.ly": true, "rb.gy": true,
	"shrtco.de": true, "tiny.cc": true, "lnkd.in": true, "goo.gl": true,
	"youtu.be": true, "amzn.to": true, "fb.me": true, "wa.me": true,
	"snipurl.com": true, "cli.gs": true, "su.pr": true, "u.to": true,
	"v.gd": true, "mcaf.ee": true, "qr.net": true, "dik.si": true,
}

// Known malicious TLDs (higher risk) 
var riskyTLDs = map[string]bool{
	".tk": true, ".ml": true, ".ga": true, ".cf": true, ".gq": true,
	".xyz": true, ".top": true, ".work": true, ".click": true, ".loan": true,
	".win": true, ".download": true, ".racing": true, ".trade": true,
	".webcam": true, ".cricket": true, ".science": true, ".party": true,
	".review": true, ".accountant": true, ".date": true, ".faith": true,
	".stream": true, ".bid": true, ".men": true,
}

// Homoglyph confusable characters map
var homoglyphs = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y',
	'х': 'x', 'А': 'A', 'В': 'B', 'С': 'C', 'Е': 'E', 'К': 'K',
	'М': 'M', 'Н': 'H', 'О': 'O', 'Р': 'P', 'Т': 'T', 'Х': 'X',
}

// Well-known legitimate domains (trusted)
var trustedDomains = map[string]bool{
	"google.com": true, "github.com": true, "wikipedia.org": true,
	"stackoverflow.com": true, "mozilla.org": true, "apple.com": true,
	"microsoft.com": true, "amazon.com": true, "youtube.com": true,
	"cloudflare.com": true, "reddit.com": true, "twitter.com": true,
	"x.com": true, "linkedin.com": true, "gitlab.com": true,
	"npm.js.org": true, "pypi.org": true, "rust-lang.org": true,
}

var rxPunycode = regexp.MustCompile(`xn--[a-z0-9]+`)
var rxIPURL = regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}`)
var rxSuspiciousKeywords = regexp.MustCompile(`(?i)(paypal|apple|google|microsoft|amazon|netflix|bank|login|signin|account|verify|secure|update|confirm|password|credential|wallet|crypto|bitcoin)`)

// ScanURL performs threat intelligence analysis on a URL
func ScanURL(ctx context.Context, rawURL string) *ThreatReport {
	start := time.Now()
	report := &ThreatReport{
		InputURL:    rawURL,
		ThreatLevel: ThreatSafe,
		ThreatScore: 0,
		Signals:     []ThreatSignal{},
		RedirectChain: []RedirectHop{},
	}

	// Normalize URL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsedInput, err := url.Parse(rawURL)
	if err != nil {
		report.Summary = "Invalid URL format."
		report.ThreatLevel = ThreatMedium
		return report
	}

	inputDomain := strings.ToLower(strings.TrimPrefix(parsedInput.Host, "www."))
	report.FinalDomain = inputDomain

	// --- Check 1: URL Shortener ---
	if urlShorteners[inputDomain] {
		report.IsShortened = true
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "URL_SHORTENER",
			Category:    "Obfuscation",
			Title:       "URL Shortener Detected",
			Description: fmt.Sprintf("This URL uses a known shortening service (%s) that masks the real destination.", inputDomain),
			Level:       ThreatLow,
		})
		report.ThreatScore += 10
	}

	// --- Check 2: Punycode / IDN Homoglyph Attack ---
	if rxPunycode.MatchString(parsedInput.Host) {
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "PUNYCODE_DOMAIN",
			Category:    "Phishing",
			Title:       "Punycode / IDN Domain Detected",
			Description: "The domain uses Punycode encoding (xn--), which can be used to spoof trusted brand names with visually similar foreign characters.",
			Level:       ThreatHigh,
		})
		report.ThreatScore += 35
	}

	// --- Check 3: Cyrillic/Homoglyph character spoofing ---
	for _, ch := range parsedInput.Host {
		if _, isHomoglyph := homoglyphs[ch]; isHomoglyph {
			report.Signals = append(report.Signals, ThreatSignal{
				Code:        "HOMOGLYPH_SPOOFING",
				Category:    "Phishing",
				Title:       "Visually Deceptive Character Detected",
				Description: "The URL contains non-ASCII characters that visually resemble Latin letters (e.g., Cyrillic 'а' vs Latin 'a'). This is a common phishing technique.",
				Level:       ThreatCritical,
			})
			report.ThreatScore += 55
			break
		}
	}

	// --- Check 4: Direct IP address as host ---
	if rxIPURL.MatchString(parsedInput.Hostname()) {
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "IP_URL",
			Category:    "Suspicious",
			Title:       "Direct IP Address in URL",
			Description: "The URL uses a raw IP address instead of a domain name. Legitimate services rarely do this; this is a common indicator of phishing or malicious hosting.",
			Level:       ThreatHigh,
		})
		report.ThreatScore += 30
	}

	// --- Check 5: Risky TLD ---
	for tld := range riskyTLDs {
		if strings.HasSuffix(inputDomain, tld) {
			report.Signals = append(report.Signals, ThreatSignal{
				Code:        "RISKY_TLD",
				Category:    "Risk Indicator",
				Title:       "High-Risk Free TLD Detected",
				Description: fmt.Sprintf("The domain uses the TLD '%s', which is heavily abused by phishers and malware distributors due to free/unregulated registration.", tld),
				Level:       ThreatMedium,
			})
			report.ThreatScore += 20
			break
		}
	}

	// --- Check 6: Suspicious brand keywords in non-trusted domain ---
	if !trustedDomains[inputDomain] {
		pathAndHost := parsedInput.Host + parsedInput.Path
		if rxSuspiciousKeywords.MatchString(pathAndHost) {
			report.Signals = append(report.Signals, ThreatSignal{
				Code:        "BRAND_SPOOFING_KEYWORDS",
				Category:    "Phishing",
				Title:       "Brand Impersonation Keywords Detected",
				Description: fmt.Sprintf("The URL contains keywords associated with major brands (e.g., paypal, apple, bank, login) in an unverified domain (%s).", inputDomain),
				Level:       ThreatHigh,
			})
			report.ThreatScore += 30
		}
	}

	// --- Check 7: Mixed script in domain ---
	hasCyrillic := false
	hasLatin := false
	for _, ch := range parsedInput.Host {
		if unicode.Is(unicode.Cyrillic, ch) {
			hasCyrillic = true
		}
		if unicode.Is(unicode.Latin, ch) {
			hasLatin = true
		}
	}
	if hasCyrillic && hasLatin {
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "MIXED_SCRIPT_DOMAIN",
			Category:    "Phishing",
			Title:       "Mixed Script Domain (Cyrillic + Latin)",
			Description: "The domain mixes Cyrillic and Latin characters, a classic IDN homograph attack vector to spoof legitimate websites.",
			Level:       ThreatCritical,
		})
		report.ThreatScore += 50
	}

	// --- Check 8: Trusted domain baseline ---
	if trustedDomains[inputDomain] && report.ThreatScore == 0 {
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "TRUSTED_DOMAIN",
			Category:    "Trust Indicator",
			Title:       "Recognized Trusted Domain",
			Description: fmt.Sprintf("'%s' is in the curated list of well-known legitimate services.", inputDomain),
			Level:       ThreatSafe,
		})
	}

	// --- HTTP probe: redirect tracing, SSL, response headers ---
	var wg sync.WaitGroup
	var probeMu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		traceRedirects(ctx, rawURL, report, &probeMu)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		probeSSL(ctx, parsedInput.Hostname(), report, &probeMu)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		probeDNS(ctx, parsedInput.Hostname(), report, &probeMu)
	}()

	wg.Wait()

	// Clamp score 0-100
	if report.ThreatScore > 100 {
		report.ThreatScore = 100
	}
	if report.ThreatScore < 0 {
		report.ThreatScore = 0
	}

	// Determine final threat level from score
	switch {
	case report.ThreatScore == 0:
		report.ThreatLevel = ThreatSafe
		report.Summary = fmt.Sprintf("No threat signals detected. '%s' appears to be a legitimate URL.", report.FinalDomain)
	case report.ThreatScore <= 20:
		report.ThreatLevel = ThreatLow
		report.Summary = fmt.Sprintf("Minor risk indicators detected. Exercise caution before sharing personal data on '%s'.", report.FinalDomain)
	case report.ThreatScore <= 45:
		report.ThreatLevel = ThreatMedium
		report.Summary = fmt.Sprintf("Moderate threat signals detected. Verify the legitimacy of '%s' before proceeding.", report.FinalDomain)
	case report.ThreatScore <= 70:
		report.ThreatLevel = ThreatHigh
		report.Summary = fmt.Sprintf("High-risk URL detected. '%s' exhibits multiple phishing/malware indicators. Avoid entering credentials.", report.FinalDomain)
	default:
		report.ThreatLevel = ThreatCritical
		report.Summary = fmt.Sprintf("CRITICAL: URL '%s' shows signs of identity spoofing, brand impersonation, or active phishing. Do NOT proceed.", report.FinalDomain)
	}

	report.ScanDuration = fmt.Sprintf("%dms", time.Since(start).Milliseconds())
	return report
}

func traceRedirects(ctx context.Context, rawURL string, report *ThreatReport, mu *sync.Mutex) {
	client := &http.Client{
		Timeout: 4 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	hops := []RedirectHop{}
	step := 0
	currentURL := rawURL

	for step < 10 {
		req, err := http.NewRequestWithContext(ctx, "GET", currentURL, nil)
		if err != nil {
			break
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SearXGo-ThreatScanner/1.0)")

		// Temporarily disable follow redirects for manual tracing
		noRedirectClient := &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := noRedirectClient.Do(req)
		if err != nil {
			break
		}
		resp.Body.Close()

		parsedCur, _ := url.Parse(currentURL)
		hop := RedirectHop{
			Step:       step + 1,
			URL:        currentURL,
			StatusCode: resp.StatusCode,
			IsHTTPS:    parsedCur != nil && parsedCur.Scheme == "https",
		}
		hops = append(hops, hop)

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				break
			}
			if !strings.HasPrefix(loc, "http") {
				base, _ := url.Parse(currentURL)
				resolved, _ := base.Parse(loc)
				loc = resolved.String()
			}
			currentURL = loc
			step++
		} else {
			// Terminal response
			mu.Lock()
			report.FinalURL = currentURL
			report.FinalStatusCode = resp.StatusCode
			report.ServerHeader = resp.Header.Get("Server")
			report.ContentType = resp.Header.Get("Content-Type")
			if parsedFinal, err2 := url.Parse(currentURL); err2 == nil {
				report.FinalDomain = strings.ToLower(strings.TrimPrefix(parsedFinal.Hostname(), "www."))
			}
			mu.Unlock()
			break
		}
	}

	// Check for HTTP→HTTPS downgrade  
	if len(hops) > 0 {
		firstHop := hops[0]
		var lastHop RedirectHop
		if len(hops) > 1 {
			lastHop = hops[len(hops)-1]
		} else {
			lastHop = firstHop
		}
		if firstHop.IsHTTPS && !lastHop.IsHTTPS {
			mu.Lock()
			report.Signals = append(report.Signals, ThreatSignal{
				Code:        "SSL_DOWNGRADE",
				Category:    "MitM Risk",
				Title:       "HTTPS to HTTP Downgrade Detected",
				Description: "The redirect chain degrades from a secure HTTPS connection to plain HTTP, exposing traffic to potential interception.",
				Level:       ThreatHigh,
			})
			report.ThreatScore += 35
			mu.Unlock()
		}
	}

	mu.Lock()
	report.RedirectChain = hops
	if len(hops) > 3 {
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "EXCESSIVE_REDIRECTS",
			Category:    "Obfuscation",
			Title:       fmt.Sprintf("Excessive Redirect Chain (%d hops)", len(hops)),
			Description: "A long redirect chain (3+) can indicate traffic laundering via ad networks, malware click-fraud chains, or affiliate link cloaking.",
			Level:       ThreatMedium,
		})
		report.ThreatScore += 15
	}
	mu.Unlock()

	// Probe final URL with a full client for more headers
	if report.FinalURL == "" {
		finalResp, err := client.Get(rawURL)
		if err == nil {
			finalResp.Body.Close()
			mu.Lock()
			report.FinalURL = finalResp.Request.URL.String()
			report.FinalStatusCode = finalResp.StatusCode
			report.ServerHeader = finalResp.Header.Get("Server")
			report.ContentType = finalResp.Header.Get("Content-Type")
			mu.Unlock()
		}
	}
}

func probeSSL(ctx context.Context, hostname string, report *ThreatReport, mu *sync.Mutex) {
	d := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 3 * time.Second},
		Config:    &tls.Config{InsecureSkipVerify: false, ServerName: hostname},
	}
	conn, err := d.DialContext(ctx, "tcp", hostname+":443")
	if err != nil {
		mu.Lock()
		report.HasSSL = false
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "NO_VALID_SSL",
			Category:    "Security",
			Title:       "No Valid SSL/TLS Certificate",
			Description: "The domain does not serve a valid TLS certificate. All data transmitted would be unencrypted and vulnerable to interception.",
			Level:       ThreatHigh,
		})
		report.ThreatScore += 30
		mu.Unlock()
		return
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return
	}

	cert := certs[0]
	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)

	mu.Lock()
	report.HasSSL = true
	report.SSLIssuer = cert.Issuer.CommonName
	report.SSLValidTo = cert.NotAfter.Format("2006-01-02")
	report.SSLDaysLeft = daysLeft
	mu.Unlock()

	if daysLeft < 14 {
		mu.Lock()
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "SSL_EXPIRING_SOON",
			Category:    "Security",
			Title:       fmt.Sprintf("SSL Certificate Expiring in %d Days", daysLeft),
			Description: "The TLS certificate is about to expire. An expired certificate breaks HTTPS and may indicate an abandoned or poorly maintained site.",
			Level:       ThreatMedium,
		})
		report.ThreatScore += 15
		mu.Unlock()
	}
}

func probeDNS(ctx context.Context, hostname string, report *ThreatReport, mu *sync.Mutex) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		mu.Lock()
		report.Signals = append(report.Signals, ThreatSignal{
			Code:        "DNS_RESOLUTION_FAILED",
			Category:    "Availability",
			Title:       "DNS Resolution Failed",
			Description: fmt.Sprintf("Could not resolve the domain '%s'. The domain may not exist, be taken down, or have misconfigured DNS.", hostname),
			Level:       ThreatMedium,
		})
		report.ThreatScore += 20
		mu.Unlock()
		return
	}

	mu.Lock()
	for _, addr := range addrs {
		if !strings.Contains(addr, ":") { // IPv4 only
			report.IPv4 = append(report.IPv4, addr)
		}
	}
	mu.Unlock()
}
