package tech

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type TechItem struct {
	Name        string `json:"name"`
	Category    string `json:"category"` // CMS, Framework, Web Server, CDN, Analytics, CSS, Language
	Version     string `json:"version,omitempty"`
	Confidence  int    `json:"confidence"` // percentage 0-100
	Description string `json:"description,omitempty"`
}

type SecurityHeaderCheck struct {
	Header      string `json:"header"`
	Present     bool   `json:"present"`
	Value       string `json:"value,omitempty"`
	Status      string `json:"status"` // "good", "warning", "missing"
	Impact      int    `json:"impact"`
	Description string `json:"description"`
}

type TechReport struct {
	TargetURL       string                `json:"target_url"`
	CleanDomain     string                `json:"clean_domain"`
	FinalURL        string                `json:"final_url"`
	StatusCode      int                   `json:"status_code"`
	ResponseTimeMs  int64                 `json:"response_time_ms"`
	SecurityGrade   string                `json:"security_grade"` // A+, A, B, C, D, F
	SecurityScore   int                   `json:"security_score"` // 0 - 100
	Technologies    []TechItem            `json:"technologies"`
	Categories      map[string][]TechItem `json:"categories"`
	SecurityHeaders []SecurityHeaderCheck `json:"security_headers"`
	ServerInfo      string                `json:"server_info,omitempty"`
	Error           string                `json:"error,omitempty"`
}

type Inspector struct {
	client *http.Client
}

func NewInspector() *Inspector {
	return &Inspector{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (ins *Inspector) Analyze(ctx context.Context, rawURL string) (*TechReport, error) {
	raw := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	report := &TechReport{
		TargetURL:       raw,
		CleanDomain:     parsed.Hostname(),
		Technologies:    make([]TechItem, 0),
		Categories:      make(map[string][]TechItem),
		SecurityHeaders: make([]SecurityHeaderCheck, 0),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 SearXGoTech/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	startTime := time.Now()
	resp, err := ins.client.Do(req)
	report.ResponseTimeMs = time.Since(startTime).Milliseconds()

	if err != nil {
		// Fallback to http if https failed
		if strings.HasPrefix(raw, "https://") {
			rawHTTP := "http://" + strings.TrimPrefix(raw, "https://")
			reqHTTP, _ := http.NewRequestWithContext(ctx, "GET", rawHTTP, nil)
			reqHTTP.Header.Set("User-Agent", req.Header.Get("User-Agent"))
			respHTTP, errHTTP := ins.client.Do(reqHTTP)
			if errHTTP == nil {
				resp = respHTTP
				err = nil
			}
		}
	}

	if err != nil {
		report.Error = fmt.Sprintf("Failed to connect to %s: %v", raw, err)
		return report, nil
	}
	defer resp.Body.Close()

	report.FinalURL = resp.Request.URL.String()
	report.StatusCode = resp.StatusCode

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // Read max 512KB for performance
	bodyStr := string(bodyBytes)
	bodyLower := strings.ToLower(bodyStr)
	headers := resp.Header

	techMap := make(map[string]TechItem)
	addTech := func(name, category, version, desc string, conf int) {
		if existing, ok := techMap[name]; ok {
			if conf > existing.Confidence {
				techMap[name] = TechItem{Name: name, Category: category, Version: version, Confidence: conf, Description: desc}
			}
		} else {
			techMap[name] = TechItem{Name: name, Category: category, Version: version, Confidence: conf, Description: desc}
		}
	}

	// 1. Headers Fingerprinting
	serverH := headers.Get("Server")
	if serverH != "" {
		report.ServerInfo = serverH
		srvLower := strings.ToLower(serverH)
		if strings.Contains(srvLower, "cloudflare") {
			addTech("Cloudflare", "CDN & Proxy", "", "Global reverse proxy & security CDN", 100)
		}
		if strings.Contains(srvLower, "nginx") {
			addTech("Nginx", "Web Server", extractVersion(serverH, "nginx"), "High performance reverse proxy & HTTP server", 100)
		}
		if strings.Contains(srvLower, "apache") {
			addTech("Apache", "Web Server", extractVersion(serverH, "Apache"), "Open-source HTTP server", 100)
		}
		if strings.Contains(srvLower, "caddy") {
			addTech("Caddy", "Web Server", "", "Modern HTTPS web server written in Go", 100)
		}
		if strings.Contains(srvLower, "litespeed") || strings.Contains(srvLower, "openlitespeed") {
			addTech("LiteSpeed", "Web Server", "", "High-speed proprietary web server", 100)
		}
		if strings.Contains(srvLower, "microsoft-iis") {
			addTech("Microsoft IIS", "Web Server", extractVersion(serverH, "Microsoft-IIS"), "Windows Server web server", 100)
		}
	}

	if headers.Get("cf-ray") != "" {
		addTech("Cloudflare", "CDN & Proxy", "", "Cloudflare Edge Network", 100)
	}
	if headers.Get("x-fastly-request-id") != "" {
		addTech("Fastly", "CDN & Proxy", "", "Fastly Edge Cloud CDN", 100)
	}
	if headers.Get("x-amz-cf-id") != "" {
		addTech("AWS CloudFront", "CDN & Proxy", "", "Amazon Web Services CDN", 100)
	}
	if headers.Get("x-vercel-id") != "" {
		addTech("Vercel", "Hosting Platform", "", "Vercel Frontend Cloud", 100)
	}
	if headers.Get("x-nf-request-id") != "" {
		addTech("Netlify", "Hosting Platform", "", "Netlify Edge Hosting", 100)
	}

	poweredBy := headers.Get("X-Powered-By")
	if poweredBy != "" {
		pbLower := strings.ToLower(poweredBy)
		if strings.Contains(pbLower, "php") {
			addTech("PHP", "Programming Language", extractVersion(poweredBy, "PHP"), "Server-side scripting language", 100)
		}
		if strings.Contains(pbLower, "express") {
			addTech("Express.js", "Backend Framework", "", "Node.js web application framework", 100)
		}
		if strings.Contains(pbLower, "next.js") {
			addTech("Next.js", "Full-Stack Framework", "", "React production framework", 100)
		}
		if strings.Contains(pbLower, "asp.net") {
			addTech("ASP.NET", "Backend Framework", "", "Microsoft .NET web framework", 100)
		}
	}

	// 2. HTML Meta Generator Inspection
	genRegex := regexp.MustCompile(`(?i)<meta\s+name=["']generator["']\s+content=["']([^"']+)["']`)
	if match := genRegex.FindStringSubmatch(bodyStr); len(match) > 1 {
		genVal := match[1]
		genLower := strings.ToLower(genVal)
		if strings.Contains(genLower, "wordpress") {
			addTech("WordPress", "CMS", extractVersion(genVal, "WordPress"), "Open-source content management system", 100)
		} else if strings.Contains(genLower, "ghost") {
			addTech("Ghost", "CMS", extractVersion(genVal, "Ghost"), "Node.js publishing platform", 100)
		} else if strings.Contains(genLower, "drupal") {
			addTech("Drupal", "CMS", extractVersion(genVal, "Drupal"), "Enterprise open-source CMS", 100)
		} else if strings.Contains(genLower, "joomla") {
			addTech("Joomla", "CMS", "", "Community-driven CMS", 100)
		} else if strings.Contains(genLower, "webflow") {
			addTech("Webflow", "Site Builder", "", "Visual web design platform", 100)
		} else if strings.Contains(genLower, "hugo") {
			addTech("Hugo", "Static Site Generator", "", "Fast static site generator in Go", 100)
		} else if strings.Contains(genLower, "astro") {
			addTech("Astro", "Frontend Framework", "", "Modern content-driven web framework", 100)
		}
	}

	// 3. CMS Fingerprinting from HTML
	if strings.Contains(bodyLower, "wp-content") || strings.Contains(bodyLower, "wp-includes") {
		addTech("WordPress", "CMS", "", "Popular blogging & website CMS", 95)
	}
	if strings.Contains(bodyLower, "cdn.shopify.com") || strings.Contains(bodyLower, "shopify.theme") {
		addTech("Shopify", "E-Commerce", "", "Cloud e-commerce platform", 100)
	}
	if strings.Contains(bodyLower, "wix.com") || strings.Contains(bodyLower, "_wix") {
		addTech("Wix", "Site Builder", "", "Cloud website builder platform", 95)
	}
	if strings.Contains(bodyLower, "squarespace.com") {
		addTech("Squarespace", "Site Builder", "", "All-in-one website solution", 95)
	}

	// 4. JS Frameworks Fingerprinting
	if strings.Contains(bodyStr, "__NEXT_DATA__") || strings.Contains(bodyStr, "/_next/static/") {
		addTech("Next.js", "Full-Stack Framework", "", "React production framework by Vercel", 100)
		addTech("React", "JavaScript Library", "", "Component-based UI library", 100)
	} else if strings.Contains(bodyStr, "_reactRootContainer") || strings.Contains(bodyStr, "data-reactroot") || strings.Contains(bodyLower, "react.production.min.js") {
		addTech("React", "JavaScript Library", "", "Component-based UI library", 90)
	}

	if strings.Contains(bodyStr, "__NUXT__") || strings.Contains(bodyStr, "/_nuxt/") {
		addTech("Nuxt.js", "Full-Stack Framework", "", "Intuitive Vue framework", 100)
		addTech("Vue.js", "JavaScript Framework", "", "Progressive JavaScript framework", 100)
	} else if strings.Contains(bodyLower, "data-v-") || strings.Contains(bodyLower, "vue.min.js") || strings.Contains(bodyStr, "__vue_app__") {
		addTech("Vue.js", "JavaScript Framework", "", "Progressive JavaScript framework", 90)
	}

	if strings.Contains(bodyLower, "ng-version") || strings.Contains(bodyLower, "ng-app") {
		addTech("Angular", "JavaScript Framework", "", "Google TypeScript-based web framework", 95)
	}

	if strings.Contains(bodyLower, "svelte-") {
		addTech("Svelte", "JavaScript Framework", "", "Cybernetically enhanced web apps", 90)
	}

	if strings.Contains(bodyLower, "x-data=") {
		addTech("Alpine.js", "JavaScript Framework", "", "Lightweight reactive framework", 90)
	}

	if strings.Contains(bodyLower, "hx-get") || strings.Contains(bodyLower, "hx-post") {
		addTech("HTMX", "JavaScript Library", "", "High-power tools for HTML", 95)
	}

	if strings.Contains(bodyLower, "jquery") || strings.Contains(bodyLower, "jquery.min.js") {
		addTech("jQuery", "JavaScript Library", "", "Fast, small feature-rich JS library", 85)
	}

	// 5. CSS Frameworks
	if strings.Contains(bodyLower, "tailwind") || strings.Contains(bodyStr, "text-") && strings.Contains(bodyStr, "flex-") && strings.Contains(bodyStr, "bg-") {
		addTech("Tailwind CSS", "CSS Framework", "", "Utility-first CSS framework", 80)
	}
	if strings.Contains(bodyLower, "bootstrap") {
		addTech("Bootstrap", "CSS Framework", "", "HTML, CSS, and JS framework", 85)
	}

	// 6. Analytics & Tag Managers
	if strings.Contains(bodyLower, "googletagmanager.com/gtag") || strings.Contains(bodyLower, "google-analytics.com") {
		addTech("Google Analytics", "Analytics", "", "Google web analytics service", 95)
	}
	if strings.Contains(bodyLower, "googletagmanager.com/gtm.js") {
		addTech("Google Tag Manager", "Tag Management", "", "Tag management system", 95)
	}
	if strings.Contains(bodyLower, "plausible.io") {
		addTech("Plausible Analytics", "Analytics", "", "Privacy-friendly, open-source analytics", 100)
	}
	if strings.Contains(bodyLower, "umami.is") {
		addTech("Umami", "Analytics", "", "Simple, privacy-focused open source analytics", 100)
	}
	if strings.Contains(bodyLower, "matomo.js") || strings.Contains(bodyLower, "piwik.js") {
		addTech("Matomo", "Analytics", "", "Google Analytics alternative protecting user data", 95)
	}
	if strings.Contains(bodyLower, "hotjar.com") {
		addTech("Hotjar", "Analytics", "", "Behavior analytics and user feedback service", 95)
	}

	// Convert tech map to slice & category breakdown
	for _, item := range techMap {
		report.Technologies = append(report.Technologies, item)
		report.Categories[item.Category] = append(report.Categories[item.Category], item)
	}

	// 7. Security Headers Audit
	score := 100
	checkHeader := func(name, desc string, impact int, goodIfPresent bool) {
		val := headers.Get(name)
		chk := SecurityHeaderCheck{
			Header:      name,
			Present:     val != "",
			Value:       val,
			Description: desc,
		}
		if val != "" {
			chk.Status = "good"
			chk.Impact = impact
		} else {
			chk.Status = "missing"
			chk.Impact = -impact
			score -= impact
		}
		report.SecurityHeaders = append(report.SecurityHeaders, chk)
	}

	checkHeader("Strict-Transport-Security", "Enforces encrypted HTTPS connections (HSTS)", 20, true)
	checkHeader("Content-Security-Policy", "Prevents XSS, data injection, and malicious script execution (CSP)", 25, true)
	checkHeader("X-Frame-Options", "Protects against clickjacking attacks inside iframes", 15, true)
	checkHeader("X-Content-Type-Options", "Prevents MIME-sniffing vulnerabilities", 10, true)
	checkHeader("Referrer-Policy", "Controls referrer privacy when navigating away", 10, true)
	checkHeader("Permissions-Policy", "Restricts camera, microphone, and browser APIs", 10, true)

	if score < 0 {
		score = 0
	}
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

func extractVersion(input, token string) string {
	idx := strings.Index(strings.ToLower(input), strings.ToLower(token))
	if idx == -1 {
		return ""
	}
	sub := input[idx+len(token):]
	sub = strings.TrimPrefix(sub, "/")
	sub = strings.TrimPrefix(sub, " ")
	parts := strings.Fields(sub)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
