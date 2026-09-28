package cve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CVEItem represents a single Common Vulnerabilities and Exposures advisory
type CVEItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
	CVSS        float64  `json:"cvss"`
	Published   string   `json:"published"`
	Vendor      string   `json:"vendor"`
	Product     string   `json:"product"`
	CWE         string   `json:"cwe,omitempty"`
	IsKEV       bool     `json:"is_kev"` // Known Exploited in the Wild
	References  []string `json:"references"`
}

// CVEResponse is the API response payload
type CVEResponse struct {
	Total    int       `json:"total"`
	Query    string    `json:"query"`
	Severity string    `json:"severity"`
	KEVOnly  bool      `json:"kev_only"`
	Items    []CVEItem `json:"items"`
}

// Service manages fetching and caching CVE records
type Service struct {
	items       []CVEItem
	lastFetched time.Time
	mu          sync.RWMutex
	client      *http.Client
}

var defaultService *Service
var once sync.Once

// GetService returns the singleton CVE service
func GetService() *Service {
	once.Do(func() {
		defaultService = &Service{
			items: seedCuratedCVEs(),
			client: &http.Client{
				Timeout: 5 * time.Second,
			},
		}
	})
	return defaultService
}

// NewService creates a new independent instance for testing
func NewService() *Service {
	return &Service{
		items: seedCuratedCVEs(),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// FetchLive updates the feed from official public sources
func (s *Service) FetchLive(ctx context.Context) error {
	s.mu.RLock()
	if time.Since(s.lastFetched) < 15*time.Minute && len(s.items) > len(seedCuratedCVEs()) {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	// Try fetching CISA KEV (Known Exploited Vulnerabilities) catalog
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "SearXGo-CVE-Intel/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		// Network dial failed, keep seeded entries
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var cisaData struct {
		Vulnerabilities []struct {
			CveID                      string `json:"cveID"`
			VendorProject              string `json:"vendorProject"`
			Product                    string `json:"product"`
			VulnerabilityName          string `json:"vulnerabilityName"`
			DateAdded                  string `json:"dateAdded"`
			ShortDescription           string `json:"shortDescription"`
			RequiredAction             string `json:"requiredAction"`
			Notes                      string `json:"notes"`
			KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
		} `json:"vulnerabilities"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cisaData); err != nil {
		return err
	}

	itemsMap := make(map[string]CVEItem)
	// Seed base entries
	for _, it := range seedCuratedCVEs() {
		itemsMap[it.ID] = it
	}

	// Add latest entries from CISA (up to 100 most recent)
	limit := 100
	if len(cisaData.Vulnerabilities) < limit {
		limit = len(cisaData.Vulnerabilities)
	}

	for i := 0; i < limit; i++ {
		vuln := cisaData.Vulnerabilities[i]
		if _, exists := itemsMap[vuln.CveID]; exists {
			cur := itemsMap[vuln.CveID]
			cur.IsKEV = true
			itemsMap[vuln.CveID] = cur
			continue
		}

		item := CVEItem{
			ID:          vuln.CveID,
			Title:       vuln.VulnerabilityName,
			Description: vuln.ShortDescription,
			Severity:    "CRITICAL",
			CVSS:        9.0,
			Published:   vuln.DateAdded,
			Vendor:      vuln.VendorProject,
			Product:     vuln.Product,
			IsKEV:       true,
			References: []string{
				fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", vuln.CveID),
				fmt.Sprintf("https://cve.mitre.org/cgi-bin/cvename.cgi?name=%s", vuln.CveID),
			},
		}
		itemsMap[item.ID] = item
	}

	merged := make([]CVEItem, 0, len(itemsMap))
	for _, it := range itemsMap {
		merged = append(merged, it)
	}

	s.mu.Lock()
	s.items = merged
	s.lastFetched = time.Now()
	s.mu.Unlock()

	return nil
}

// GetCVEs filters items by query, severity, and KEV flag
func (s *Service) GetCVEs(ctx context.Context, query string, severity string, kevOnly bool) CVEResponse {
	// Attempt background live refresh non-blockingly
	go func() {
		_ = s.FetchLive(context.Background())
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	sev := strings.ToUpper(strings.TrimSpace(severity))

	var results []CVEItem
	for _, item := range s.items {
		if sev != "" && sev != "ALL" && item.Severity != sev {
			continue
		}
		if kevOnly && !item.IsKEV {
			continue
		}
		if q != "" {
			match := strings.Contains(strings.ToLower(item.ID), q) ||
				strings.Contains(strings.ToLower(item.Title), q) ||
				strings.Contains(strings.ToLower(item.Description), q) ||
				strings.Contains(strings.ToLower(item.Vendor), q) ||
				strings.Contains(strings.ToLower(item.Product), q) ||
				strings.Contains(strings.ToLower(item.CWE), q)
			if !match {
				continue
			}
		}
		results = append(results, item)
	}

	return CVEResponse{
		Total:    len(results),
		Query:    query,
		Severity: severity,
		KEVOnly:  kevOnly,
		Items:    results,
	}
}

// seedCuratedCVEs provides immediate high-profile zero-days and critical CVEs
func seedCuratedCVEs() []CVEItem {
	return []CVEItem{
		{
			ID:          "CVE-2024-38077",
			Title:       "Windows Remote Desktop Licensing Service RCE (MadLicence)",
			Description: "Underauthenticated remote code execution vulnerability in the Windows Remote Desktop Licensing Service allows an attacker to send specially crafted packets to achieve SYSTEM execution.",
			Severity:    "CRITICAL",
			CVSS:        9.8,
			Published:   "2024-07-09",
			Vendor:      "Microsoft",
			Product:     "Windows Server 2008-2025",
			CWE:         "CWE-190: Integer Overflow",
			IsKEV:       true,
			References: []string{
				"https://msrc.microsoft.com/update-guide/vulnerability/CVE-2024-38077",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-38077",
			},
		},
		{
			ID:          "CVE-2024-6387",
			Title:       "OpenSSH regreSSHion Remote Code Execution Vulnerability",
			Description: "Signal handler race condition in OpenSSH's server (sshd) allows unauthenticated remote code execution as root on glibc-based Linux systems.",
			Severity:    "CRITICAL",
			CVSS:        8.1,
			Published:   "2024-07-01",
			Vendor:      "OpenBSD",
			Product:     "OpenSSH",
			CWE:         "CWE-362: Race Condition",
			IsKEV:       true,
			References: []string{
				"https://www.qualys.com/2024/07/01/cve-2024-6387/regresshion.txt",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-6387",
			},
		},
		{
			ID:          "CVE-2024-3094",
			Title:       "XZ Utils Liblzma Upstream Supply Chain Backdoor",
			Description: "Malicious code was discovered in upstream xz/liblzma tarballs leading to unauthorized SSH authentication bypass and remote code execution.",
			Severity:    "CRITICAL",
			CVSS:        10.0,
			Published:   "2024-03-29",
			Vendor:      "Tukaani",
			Product:     "XZ Utils / liblzma",
			CWE:         "CWE-506: Embedded Malicious Code",
			IsKEV:       true,
			References: []string{
				"https://tukaani.org/xz-backdoor/",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-3094",
			},
		},
		{
			ID:          "CVE-2024-3400",
			Title:       "Palo Alto PAN-OS GlobalProtect Command Injection",
			Description: "Command injection vulnerability in the GlobalProtect feature of Palo Alto Networks PAN-OS software allows an unauthenticated attacker to execute arbitrary code with root privileges.",
			Severity:    "CRITICAL",
			CVSS:        10.0,
			Published:   "2024-04-12",
			Vendor:      "Palo Alto Networks",
			Product:     "PAN-OS GlobalProtect",
			CWE:         "CWE-77: Command Injection",
			IsKEV:       true,
			References: []string{
				"https://security.paloaltonetworks.com/CVE-2024-3400",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-3400",
			},
		},
		{
			ID:          "CVE-2024-21887",
			Title:       "Ivanti Connect Secure & Policy Secure Command Injection",
			Description: "Command injection vulnerability in web components of Ivanti Connect Secure allows an authenticated administrator to send specially crafted requests and execute arbitrary commands.",
			Severity:    "CRITICAL",
			CVSS:        9.1,
			Published:   "2024-01-12",
			Vendor:      "Ivanti",
			Product:     "Connect Secure (ICS)",
			CWE:         "CWE-78: OS Command Injection",
			IsKEV:       true,
			References: []string{
				"https://forums.ivanti.com/s/article/CVE-2023-46805-CVE-2024-21887",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-21887",
			},
		},
		{
			ID:          "CVE-2024-4577",
			Title:       "PHP CGI Argument Injection Vulnerability on Windows",
			Description: "Best-fit character encoding mapping flaw in PHP-CGI on Windows systems allows remote attackers to bypass CVE-2012-1823 protection and execute arbitrary code.",
			Severity:    "CRITICAL",
			CVSS:        9.8,
			Published:   "2024-06-06",
			Vendor:      "PHP Group",
			Product:     "PHP (Windows CGI)",
			CWE:         "CWE-88: Argument Injection",
			IsKEV:       true,
			References: []string{
				"https://devco.re/blog/2024/06/06/security-alert-cve-2024-4577-php-cgi-argument-injection-vulnerability-en/",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-4577",
			},
		},
		{
			ID:          "CVE-2024-27198",
			Title:       "JetBrains TeamCity Authentication Bypass RCE",
			Description: "Authentication bypass vulnerability in the web component of JetBrains TeamCity Server allows unauthenticated remote attackers to perform administrative actions and achieve RCE.",
			Severity:    "CRITICAL",
			CVSS:        9.8,
			Published:   "2024-03-04",
			Vendor:      "JetBrains",
			Product:     "TeamCity Server",
			CWE:         "CWE-288: Authentication Bypass",
			IsKEV:       true,
			References: []string{
				"https://blog.jetbrains.com/teamcity/2024/03/additional-critical-security-issues-affecting-teamcity-on-premises-cve-2024-27198-and-cve-2024-27199/",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-27198",
			},
		},
		{
			ID:          "CVE-2023-4863",
			Title:       "Google Chrome & libwebp Heap Buffer Overflow",
			Description: "Heap buffer overflow in WebP image processing in Google Chrome, Electron, and libwebp allows remote attackers to execute arbitrary code via a crafted WebP lossless file.",
			Severity:    "CRITICAL",
			CVSS:        8.8,
			Published:   "2023-09-12",
			Vendor:      "Google",
			Product:     "Chrome & libwebp",
			CWE:         "CWE-122: Heap-based Buffer Overflow",
			IsKEV:       true,
			References: []string{
				"https://chromereleases.googleblog.com/2023/09/stable-channel-update-for-desktop_11.html",
				"https://nvd.nist.gov/vuln/detail/CVE-2023-4863",
			},
		},
		{
			ID:          "CVE-2021-44228",
			Title:       "Apache Log4j2 JNDI Remote Code Execution (Log4Shell)",
			Description: "JNDI lookup features in Apache Log4j2 did not protect against attacker-controlled LDAP and other JNDI related endpoints, allowing full remote code execution.",
			Severity:    "CRITICAL",
			CVSS:        10.0,
			Published:   "2021-12-10",
			Vendor:      "Apache Software Foundation",
			Product:     "Log4j",
			CWE:         "CWE-502: Deserialization of Untrusted Data",
			IsKEV:       true,
			References: []string{
				"https://logging.apache.org/log4j/2.x/security.html",
				"https://nvd.nist.gov/vuln/detail/CVE-2021-44228",
			},
		},
		{
			ID:          "CVE-2024-23897",
			Title:       "Jenkins CLI Arbitrary File Read and RCE",
			Description: "The args4j command-line parser used in Jenkins expands file paths after the @ character, allowing unauthenticated attackers to read arbitrary files from the Jenkins controller file system.",
			Severity:    "CRITICAL",
			CVSS:        9.8,
			Published:   "2024-01-24",
			Vendor:      "Jenkins",
			Product:     "Jenkins Core",
			CWE:         "CWE-200: Exposure of Sensitive Information",
			IsKEV:       true,
			References: []string{
				"https://www.jenkins.io/security/advisory/2024-01-24/",
				"https://nvd.nist.gov/vuln/detail/CVE-2024-23897",
			},
		},
	}
}
