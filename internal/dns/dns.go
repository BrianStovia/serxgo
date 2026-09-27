package dns

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Record represents a single DNS record
type Record struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

// ResolverResult represents DNS resolution from a specific resolver
type ResolverResult struct {
	ResolverName string   `json:"resolver_name"`
	ResolverIP   string   `json:"resolver_ip"`
	Location     string   `json:"location"`
	Status       string   `json:"status"` // "OK", "NXDOMAIN", "TIMEOUT", "ERROR"
	LatencyMs    int64    `json:"latency_ms"`
	Records      []Record `json:"records"`
	Error        string   `json:"error,omitempty"`
}

// PropagationReport contains all propagation test results
type PropagationReport struct {
	Domain      string           `json:"domain"`
	RecordType  string           `json:"record_type"`
	CheckedAt   string           `json:"checked_at"`
	Resolvers   []ResolverResult `json:"resolvers"`
	AllRecords  []Record         `json:"all_records,omitempty"`
	IsConsistent bool            `json:"is_consistent"`
}

// ResolverConfig holds configuration for public resolvers
type ResolverConfig struct {
	Name     string
	IP       string
	Location string
	DoHURL   string
}

var PublicResolvers = []ResolverConfig{
	{
		Name:     "Cloudflare",
		IP:       "1.1.1.1",
		Location: "Global Anycast",
		DoHURL:   "https://cloudflare-dns.com/dns-query",
	},
	{
		Name:     "Google Public DNS",
		IP:       "8.8.8.8",
		Location: "Global Anycast",
		DoHURL:   "https://dns.google/resolve",
	},
	{
		Name:     "Quad9 (Secure)",
		IP:       "9.9.9.9",
		Location: "Zurich, Global",
		DoHURL:   "https://dns.quad9.net/dns-query",
	},
	{
		Name:     "AdGuard DNS",
		IP:       "94.140.14.14",
		Location: "Cyprus, Global",
		DoHURL:   "https://dns.adguard-dns.com/dns-query",
	},
	{
		Name:     "OpenDNS (Cisco)",
		IP:       "208.67.222.222",
		Location: "USA Anycast",
		DoHURL:   "https://doh.opendns.com/dns-query",
	},
}

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
				MaxIdleConns:    20,
			},
		},
	}
}

// CleanDomain extracts pure domain from input
func CleanDomain(input string) string {
	d := strings.TrimSpace(strings.ToLower(input))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	if idx := strings.Index(d, "/"); idx != -1 {
		d = d[:idx]
	}
	if idx := strings.Index(d, ":"); idx != -1 {
		d = d[:idx]
	}
	return strings.Trim(d, ".")
}

// CheckPropagation queries public resolvers concurrently for the given domain and record type
func (s *Service) CheckPropagation(ctx context.Context, domain string, recordType string) (*PropagationReport, error) {
	domain = CleanDomain(domain)
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	if recordType == "" {
		recordType = "A"
	}

	report := &PropagationReport{
		Domain:     domain,
		RecordType: recordType,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		Resolvers:  make([]ResolverResult, len(PublicResolvers)),
	}

	var wg sync.WaitGroup
	for i, cfg := range PublicResolvers {
		wg.Add(1)
		go func(idx int, c ResolverConfig) {
			defer wg.Done()
			report.Resolvers[idx] = s.queryDoH(ctx, c, domain, recordType)
		}(i, cfg)
	}
	wg.Wait()

	// Also perform complete local resolver lookup for ALL records if recordType is "ALL" or "A"
	report.AllRecords = s.queryAllRecordsLocal(ctx, domain)

	// Check consistency
	var firstVal string
	consistent := true
	count := 0
	for _, res := range report.Resolvers {
		if res.Status == "OK" && len(res.Records) > 0 {
			val := res.Records[0].Value
			if firstVal == "" {
				firstVal = val
			} else if firstVal != val {
				consistent = false
			}
			count++
		}
	}
	report.IsConsistent = (count >= 2 && consistent)

	return report, nil
}

type dohResponse struct {
	Status int `json:"Status"` // 0 = NOERROR, 3 = NXDOMAIN
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
	Authority []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Authority"`
}

var typeIntToString = map[int]string{
	1:  "A",
	28: "AAAA",
	15: "MX",
	16: "TXT",
	5:  "CNAME",
	2:  "NS",
	6:  "SOA",
	257: "CAA",
	12: "PTR",
}

func (s *Service) queryDoH(ctx context.Context, cfg ResolverConfig, domain, recordType string) ResolverResult {
	start := time.Now()
	res := ResolverResult{
		ResolverName: cfg.Name,
		ResolverIP:   cfg.IP,
		Location:     cfg.Location,
		Status:       "TIMEOUT",
		Records:      []Record{},
	}

	reqURL := fmt.Sprintf("%s?name=%s&type=%s", cfg.DoHURL, url.QueryEscape(domain), url.QueryEscape(recordType))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		res.Status = "ERROR"
		res.Error = err.Error()
		res.LatencyMs = time.Since(start).Milliseconds()
		return res
	}
	req.Header.Set("Accept", "application/dns-json")
	req.Header.Set("User-Agent", "SearXGo-DNS-Inspector/1.0")

	resp, err := s.httpClient.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Status = "ERROR"
		res.Error = "Connection error"
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		res.Status = "ERROR"
		res.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return res
	}

	var dohResp dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&dohResp); err != nil {
		res.Status = "ERROR"
		res.Error = "Invalid DNS-JSON response"
		return res
	}

	if dohResp.Status == 3 {
		res.Status = "NXDOMAIN"
		return res
	}

	if dohResp.Status != 0 {
		res.Status = fmt.Sprintf("RCODE_%d", dohResp.Status)
		return res
	}

	for _, ans := range dohResp.Answer {
		tStr, ok := typeIntToString[ans.Type]
		if !ok {
			tStr = fmt.Sprintf("TYPE%d", ans.Type)
		}
		cleanData := strings.Trim(ans.Data, "\"")
		res.Records = append(res.Records, Record{
			Type:  tStr,
			Name:  strings.TrimSuffix(ans.Name, "."),
			Value: cleanData,
			TTL:   ans.TTL,
		})
	}

	if len(res.Records) > 0 {
		res.Status = "OK"
	} else {
		res.Status = "NO_DATA"
	}

	return res
}

func (s *Service) queryAllRecordsLocal(ctx context.Context, domain string) []Record {
	var records []Record
	resolver := net.DefaultResolver

	// A records
	if ips, err := resolver.LookupIP(ctx, "ip4", domain); err == nil {
		for _, ip := range ips {
			records = append(records, Record{Type: "A", Name: domain, Value: ip.String()})
		}
	}

	// AAAA records
	if ips, err := resolver.LookupIP(ctx, "ip6", domain); err == nil {
		for _, ip := range ips {
			records = append(records, Record{Type: "AAAA", Name: domain, Value: ip.String()})
		}
	}

	// MX records
	if mxs, err := resolver.LookupMX(ctx, domain); err == nil {
		for _, mx := range mxs {
			records = append(records, Record{
				Type:     "MX",
				Name:     domain,
				Value:    strings.TrimSuffix(mx.Host, "."),
				Priority: int(mx.Pref),
			})
		}
	}

	// TXT records
	if txts, err := resolver.LookupTXT(ctx, domain); err == nil {
		for _, txt := range txts {
			records = append(records, Record{Type: "TXT", Name: domain, Value: txt})
		}
	}

	// NS records
	if nss, err := resolver.LookupNS(ctx, domain); err == nil {
		for _, ns := range nss {
			records = append(records, Record{Type: "NS", Name: domain, Value: strings.TrimSuffix(ns.Host, ".")})
		}
	}

	// CNAME
	if cname, err := resolver.LookupCNAME(ctx, domain); err == nil {
		trimmed := strings.TrimSuffix(cname, ".")
		if trimmed != domain {
			records = append(records, Record{Type: "CNAME", Name: domain, Value: trimmed})
		}
	}

	return records
}
