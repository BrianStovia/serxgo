package subdomains

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type SubdomainItem struct {
	Subdomain string   `json:"subdomain"`
	IPs       []string `json:"ips,omitempty"`
	IsActive  bool     `json:"is_active"`
}

type SubdomainReport struct {
	Domain     string          `json:"domain"`
	TotalFound int             `json:"total_found"`
	ActiveNum  int             `json:"active_num"`
	Items      []SubdomainItem `json:"items"`
	DurationMs int64           `json:"duration_ms"`
	Error      string          `json:"error,omitempty"`
}

type Finder struct {
	client *http.Client
}

func NewFinder() *Finder {
	return &Finder{
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

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
	return strings.Trim(raw, ".")
}

func (f *Finder) Enumerate(ctx context.Context, targetDomain string) (*SubdomainReport, error) {
	domain := CleanDomain(targetDomain)
	if domain == "" {
		return nil, fmt.Errorf("domain is empty")
	}

	start := time.Now()
	report := &SubdomainReport{
		Domain: domain,
		Items:  make([]SubdomainItem, 0),
	}

	rawSet := make(map[string]bool)

	// 1. Query crt.sh (Certificate Transparency Log)
	crtURL := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)
	req, err := http.NewRequestWithContext(ctx, "GET", crtURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo/1.0 (+https://github.com/BrianStovia/serxgo)")
		req.Header.Set("Accept", "application/json")

		resp, errDo := f.client.Do(req)
		if errDo == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var entries []struct {
				NameValue string `json:"name_value"`
			}
			if jsonErr := json.NewDecoder(resp.Body).Decode(&entries); jsonErr == nil {
				for _, entry := range entries {
					// name_value can contain multiple names separated by \n
					names := strings.Split(entry.NameValue, "\n")
					for _, name := range names {
						clean := strings.ToLower(strings.TrimSpace(name))
						clean = strings.TrimPrefix(clean, "*.")
						if strings.HasSuffix(clean, "."+domain) || clean == domain {
							if !strings.Contains(clean, " ") && !strings.Contains(clean, "@") {
								rawSet[clean] = true
							}
						}
					}
				}
			}
		}
	}

	// 2. Fallback to Certspotter if crt.sh was empty or errored
	if len(rawSet) == 0 {
		spotterURL := fmt.Sprintf("https://api.certspotter.com/v1/issuances?domain=%s&include_subdomains=true&expand=dns_names", domain)
		reqSpotter, errSpotter := http.NewRequestWithContext(ctx, "GET", spotterURL, nil)
		if errSpotter == nil {
			reqSpotter.Header.Set("User-Agent", "SearXGo/1.0")
			respSpotter, errDoSpotter := f.client.Do(reqSpotter)
			if errDoSpotter == nil && respSpotter.StatusCode == http.StatusOK {
				defer respSpotter.Body.Close()
				var records []struct {
					DNSNames []string `json:"dns_names"`
				}
				if jsonErr := json.NewDecoder(respSpotter.Body).Decode(&records); jsonErr == nil {
					for _, rec := range records {
						for _, name := range rec.DNSNames {
							clean := strings.ToLower(strings.TrimSpace(name))
							clean = strings.TrimPrefix(clean, "*.")
							if strings.HasSuffix(clean, "."+domain) || clean == domain {
								rawSet[clean] = true
							}
						}
					}
				}
			}
		}
	}

	// Always ensure root domain and standard prefixes exist in checking list
	rawSet[domain] = true
	commonPrefixes := []string{"www", "api", "mail", "dev", "app", "m", "portal", "admin", "blog", "vpn"}
	for _, p := range commonPrefixes {
		rawSet[p+"."+domain] = true
	}

	// Convert to slice & sort
	var subList []string
	for sub := range rawSet {
		subList = append(subList, sub)
	}
	sort.Strings(subList)

	// 3. Fast Concurrent DNS Resolution (probe up to 80 subdomains concurrently)
	results := make([]SubdomainItem, len(subList))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 25) // 25 concurrent DNS lookups
	resolver := &net.Resolver{
		PreferGo: true,
	}

	for i, sub := range subList {
		wg.Add(1)
		go func(idx int, target string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			resolveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			ips, errLookup := resolver.LookupHost(resolveCtx, target)
			if errLookup == nil && len(ips) > 0 {
				results[idx] = SubdomainItem{
					Subdomain: target,
					IPs:       ips,
					IsActive:  true,
				}
			} else {
				results[idx] = SubdomainItem{
					Subdomain: target,
					IsActive:  false,
				}
			}
		}(i, sub)
	}
	wg.Wait()

	activeCount := 0
	filtered := make([]SubdomainItem, 0, len(results))
	for _, item := range results {
		if item.Subdomain == "" {
			continue
		}
		if item.IsActive {
			activeCount++
		}
		filtered = append(filtered, item)
	}

	// Sort active first, then alphabetically
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IsActive != filtered[j].IsActive {
			return filtered[i].IsActive // active first
		}
		return filtered[i].Subdomain < filtered[j].Subdomain
	})

	report.Items = filtered
	report.TotalFound = len(filtered)
	report.ActiveNum = activeCount
	report.DurationMs = time.Since(start).Milliseconds()

	return report, nil
}
