package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DNSBLResult stores blacklist check status
type DNSBLResult struct {
	List       string `json:"list"`
	IsListed   bool   `json:"is_listed"`
	StatusCode string `json:"status_code,omitempty"`
}

// IPIntelReport holds full intelligence report for an IP or domain
type IPIntelReport struct {
	Query       string        `json:"query"`
	IP          string        `json:"ip"`
	Hostname    string        `json:"hostname"`
	ASN         string        `json:"asn"`
	ISP         string        `json:"isp"`
	Org         string        `json:"org"`
	Country     string        `json:"country"`
	CountryCode string        `json:"country_code"`
	Region      string        `json:"region"`
	City        string        `json:"city"`
	Latitude    float64       `json:"latitude"`
	Longitude   float64       `json:"longitude"`
	Timezone    string        `json:"timezone"`
	IsHosting   bool          `json:"is_hosting"`
	IsPrivate   bool          `json:"is_private"`
	DNSBLStatus []DNSBLResult `json:"dnsbl_status"`
	DurationMs  int64         `json:"duration_ms"`
	Error       string        `json:"error,omitempty"`
}

var hostingKeywords = []string{
	"amazon", "aws", "cloudflare", "digitalocean", "google", "microsoft", "azure",
	"ovh", "hetzner", "linode", "vultr", "oracle", "fastly", "akamai", "alibaba",
	"tencent", "leaseweb", "choopa", "hostinger", "scaleway", "cogent",
}

// Investigate resolves and probes IP intelligence
func Investigate(ctx context.Context, target string) *IPIntelReport {
	start := time.Now()
	target = strings.TrimSpace(target)
	report := &IPIntelReport{
		Query:       target,
		DNSBLStatus: []DNSBLResult{},
	}

	if target == "" {
		report.Error = "Empty target query"
		return report
	}

	// Clean protocol if provided
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}
	if idx := strings.Index(target, ":"); idx != -1 {
		// Could be host:port or IPv6
		if !strings.Contains(target, "::") && strings.Count(target, ":") == 1 {
			target = target[:idx]
		}
	}

	ipObj := net.ParseIP(target)
	var resolvedIP string

	if ipObj != nil {
		resolvedIP = ipObj.String()
	} else {
		// Target is a domain name, resolve to IP
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", target)
		if err != nil || len(ips) == 0 {
			// Try general lookup
			ips, err = net.DefaultResolver.LookupIP(ctx, "ip", target)
		}
		if err != nil || len(ips) == 0 {
			report.Error = fmt.Sprintf("Could not resolve domain '%s' to IP", target)
			return report
		}
		resolvedIP = ips[0].String()
		report.Hostname = target
	}

	report.IP = resolvedIP
	ipObj = net.ParseIP(resolvedIP)

	// Check if private IP
	if ipObj != nil && (ipObj.IsPrivate() || ipObj.IsLoopback() || ipObj.IsLinkLocalUnicast()) {
		report.IsPrivate = true
		report.ISP = "Private Network / Loopback"
		report.Org = "Local / RFC 1918"
		report.Country = "Internal"
		report.City = "Localhost"
		report.DurationMs = time.Since(start).Milliseconds()
		return report
	}

	// Concurrent tasks: Reverse DNS, Geo/ASN API, DNSBL
	var wg sync.WaitGroup

	// Task 1: Reverse DNS PTR
	if report.Hostname == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			names, err := net.DefaultResolver.LookupAddr(ctx, resolvedIP)
			if err == nil && len(names) > 0 {
				report.Hostname = strings.TrimSuffix(names[0], ".")
			}
		}()
	}

	// Task 2: Geo & ASN Lookup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fetchGeoASN(ctx, resolvedIP, report)
	}()

	// Task 3: DNSBL Reputation check
	wg.Add(1)
	go func() {
		defer wg.Done()
		checkDNSBL(ctx, resolvedIP, report)
	}()

	wg.Wait()

	// Check if hosting provider
	combined := strings.ToLower(report.ISP + " " + report.Org + " " + report.ASN)
	for _, kw := range hostingKeywords {
		if strings.Contains(combined, kw) {
			report.IsHosting = true
			break
		}
	}

	report.DurationMs = time.Since(start).Milliseconds()
	return report
}

// fetchGeoASN contacts public IP intelligence APIs
func fetchGeoASN(ctx context.Context, ip string, report *IPIntelReport) {
	client := &http.Client{Timeout: 3 * time.Second}

	// Primary: ip-api.com (reliable, fast, returns ASN + geo without key)
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,region,regionName,city,lat,lon,timezone,isp,org,as", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err == nil {
		req.Header.Set("User-Agent", "SearXGo-IPIntel/1.0")
		resp, err2 := client.Do(req)
		if err2 == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var data struct {
				Status      string  `json:"status"`
				Country     string  `json:"country"`
				CountryCode string  `json:"countryCode"`
				RegionName  string  `json:"regionName"`
				City        string  `json:"city"`
				Lat         float64 `json:"lat"`
				Lon         float64 `json:"lon"`
				Timezone    string  `json:"timezone"`
				ISP         string  `json:"isp"`
				Org         string  `json:"org"`
				AS          string  `json:"as"`
			}
			if err3 := json.NewDecoder(resp.Body).Decode(&data); err3 == nil && data.Status == "success" {
				report.Country = data.Country
				report.CountryCode = data.CountryCode
				report.Region = data.RegionName
				report.City = data.City
				report.Latitude = data.Lat
				report.Longitude = data.Lon
				report.Timezone = data.Timezone
				report.ISP = data.ISP
				report.Org = data.Org
				report.ASN = data.AS
				return
			}
		}
	}

	// Fallback: freeipapi.com
	fallbackURL := fmt.Sprintf("https://freeipapi.com/api/json/%s", ip)
	reqFallback, err := http.NewRequestWithContext(ctx, "GET", fallbackURL, nil)
	if err == nil {
		resp, err2 := client.Do(reqFallback)
		if err2 == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var fbData struct {
				CountryName string  `json:"countryName"`
				CountryCode string  `json:"countryCode"`
				RegionName  string  `json:"regionName"`
				CityName    string  `json:"cityName"`
				Latitude    float64 `json:"latitude"`
				Longitude   float64 `json:"longitude"`
				TimeZone    string  `json:"timeZone"`
			}
			if err3 := json.NewDecoder(resp.Body).Decode(&fbData); err3 == nil {
				report.Country = fbData.CountryName
				report.CountryCode = fbData.CountryCode
				report.Region = fbData.RegionName
				report.City = fbData.CityName
				report.Latitude = fbData.Latitude
				report.Longitude = fbData.Longitude
				report.Timezone = fbData.TimeZone
			}
		}
	}
}

// checkDNSBL checks popular DNSBLs for spam/malware reputation
func checkDNSBL(ctx context.Context, ip string, report *IPIntelReport) {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return // IPv4 only for standard reverse DNSBL
	}
	reversed := fmt.Sprintf("%s.%s.%s.%s", parts[3], parts[2], parts[1], parts[0])

	lists := []string{
		"zen.spamhaus.org",
		"bl.spamcop.net",
		"dnsbl.sorbs.net",
	}

	var mu sync.Mutex
	var subWg sync.WaitGroup

	for _, dnsbl := range lists {
		subWg.Add(1)
		go func(listName string) {
			defer subWg.Done()
			lookupHost := fmt.Sprintf("%s.%s", reversed, listName)
			// Short timeout for DNSBL lookup
			dnsCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			defer cancel()

			res, err := net.DefaultResolver.LookupHost(dnsCtx, lookupHost)
			isListed := err == nil && len(res) > 0
			code := ""
			if isListed {
				code = res[0]
			}

			mu.Lock()
			report.DNSBLStatus = append(report.DNSBLStatus, DNSBLResult{
				List:       listName,
				IsListed:   isListed,
				StatusCode: code,
			})
			mu.Unlock()
		}(dnsbl)
	}

	subWg.Wait()
}
