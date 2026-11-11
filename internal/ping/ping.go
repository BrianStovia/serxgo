package ping

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"searxgo/internal/netguard"
)

type PortResult struct {
	Port      int    `json:"port"`
	Service   string `json:"service"`
	IsOpen    bool   `json:"is_open"`
	LatencyMs int64  `json:"latency_ms"`
	Status    string `json:"status"` // "OPEN", "CLOSED", "TIMEOUT"
}

type TLSCertInfo struct {
	Issuer        string   `json:"issuer"`
	Subject       string   `json:"subject"`
	ValidFrom     string   `json:"valid_from"`
	ValidTo       string   `json:"valid_to"`
	DaysRemaining int      `json:"days_remaining"`
	IsExpired     bool     `json:"is_expired"`
	DNSNames      []string `json:"dns_names,omitempty"`
}

type PingReport struct {
	Target         string       `json:"target"`
	Host           string       `json:"host"`
	ResolvedIP     string       `json:"resolved_ip"`
	IsUp           bool         `json:"is_up"`
	Verdict        string       `json:"verdict"` // "UP", "DEGRADED", "DOWN"
	HTTPStatusCode int          `json:"http_status_code"`
	HTTPStatusText string       `json:"http_status_text"`
	HTTPLatencyMs  int64        `json:"http_latency_ms"`
	FinalURL       string       `json:"final_url"`
	ServerHeader   string       `json:"server_header,omitempty"`
	ContentType    string       `json:"content_type,omitempty"`
	RedirectHops   int          `json:"redirect_hops"`
	TLSCert        *TLSCertInfo `json:"tls_cert,omitempty"`
	PortResults    []PortResult `json:"port_results"`
	CheckedAt      string       `json:"checked_at"`
	ErrorMessage   string       `json:"error_message,omitempty"`
}

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	return &Service{
		httpClient: netguard.NewHTTPClient(5*time.Second, 5),
	}
}

var CommonPorts = []struct {
	Port    int
	Service string
}{
	{80, "HTTP"},
	{443, "HTTPS"},
	{22, "SSH"},
	{21, "FTP"},
	{25, "SMTP"},
	{53, "DNS"},
	{3306, "MySQL"},
	{5432, "PostgreSQL"},
	{8080, "HTTP-Alt"},
}

func cleanHost(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, raw
	}
	host := u.Hostname()
	return host, raw
}

func (s *Service) Check(ctx context.Context, targetInput string) (*PingReport, error) {
	host, targetURL := cleanHost(targetInput)
	if host == "" {
		return nil, fmt.Errorf("target host cannot be empty")
	}
	parsedTarget, err := netguard.ValidatePublicURL(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid or restricted target: %w", err)
	}
	host = parsedTarget.Hostname()
	targetURL = parsedTarget.String()

	report := &PingReport{
		Target:      targetURL,
		Host:        host,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
		PortResults: make([]PortResult, 0),
	}

	// Resolve IP address
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err == nil && len(ips) > 0 {
		report.ResolvedIP = ips[0].String()
	}

	// 1. HTTP Probe
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 SearXGo-Ping/1.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		hops := 0
		client := netguard.NewHTTPClient(6*time.Second, 5)
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			hops = len(via)
			if len(via) > 5 {
				return http.ErrUseLastResponse
			}
			_, err := netguard.ValidatePublicURL(req.URL.String())
			return err
		}

		resp, err := client.Do(req)
		report.HTTPLatencyMs = time.Since(start).Milliseconds()

		if err != nil {
			// If HTTPS failed, try HTTP as fallback
			if strings.HasPrefix(targetURL, "https://") {
				httpFallback := "http://" + host
				startFallback := time.Now()
				if reqFb, errFb := http.NewRequestWithContext(ctx, "GET", httpFallback, nil); errFb == nil {
					if respFb, errFb2 := client.Do(reqFb); errFb2 == nil {
						defer respFb.Body.Close()
						report.IsUp = true
						report.Verdict = "UP"
						report.HTTPStatusCode = respFb.StatusCode
						report.HTTPStatusText = respFb.Status
						report.HTTPLatencyMs = time.Since(startFallback).Milliseconds()
						report.FinalURL = respFb.Request.URL.String()
						report.ServerHeader = respFb.Header.Get("Server")
						report.ContentType = respFb.Header.Get("Content-Type")
						resp = nil
					}
				}
			}

			if !report.IsUp {
				report.IsUp = false
				report.Verdict = "DOWN"
				report.ErrorMessage = err.Error()
			}
		} else {
			defer resp.Body.Close()
			report.IsUp = true
			report.HTTPStatusCode = resp.StatusCode
			report.HTTPStatusText = resp.Status
			report.FinalURL = resp.Request.URL.String()
			report.ServerHeader = resp.Header.Get("Server")
			report.ContentType = resp.Header.Get("Content-Type")
			report.RedirectHops = hops

			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				report.Verdict = "UP"
			} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				report.Verdict = "UP" // Server responded, client error
			} else {
				report.Verdict = "DEGRADED" // 5xx Server Error
			}
		}
	}

	// 2. TLS Certificate Probe
	tlsCtx, cancelTLS := context.WithTimeout(ctx, 3*time.Second)
	conn, err := netguard.DialContext(tlsCtx, "tcp", net.JoinHostPort(host, "443"))
	var tlsConn *tls.Conn
	if err == nil {
		tlsConn = tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		err = tlsConn.HandshakeContext(tlsCtx)
		if err != nil {
			conn.Close()
			tlsConn = nil
		}
	}
	cancelTLS()
	if err == nil {
		defer tlsConn.Close()
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)
			report.TLSCert = &TLSCertInfo{
				Issuer:        cert.Issuer.CommonName,
				Subject:       cert.Subject.CommonName,
				ValidFrom:     cert.NotBefore.Format("2006-01-02"),
				ValidTo:       cert.NotAfter.Format("2006-01-02"),
				DaysRemaining: daysRemaining,
				IsExpired:     time.Now().After(cert.NotAfter),
				DNSNames:      cert.DNSNames,
			}
		}
	}

	// 3. Concurrent TCP Port Scanner
	var wg sync.WaitGroup
	var portMu sync.Mutex
	for _, p := range CommonPorts {
		wg.Add(1)
		go func(portNum int, serviceName string) {
			defer wg.Done()
			pRes := s.scanPort(ctx, host, portNum, serviceName)
			portMu.Lock()
			report.PortResults = append(report.PortResults, pRes)
			portMu.Unlock()
		}(p.Port, p.Service)
	}
	wg.Wait()

	// If HTTP failed but port 80 or 443 was open, mark as DEGRADED rather than purely DOWN
	if !report.IsUp {
		hasOpenPort := false
		for _, pr := range report.PortResults {
			if pr.IsOpen {
				hasOpenPort = true
				break
			}
		}
		if hasOpenPort {
			report.IsUp = true
			report.Verdict = "DEGRADED"
		}
	}

	return report, nil
}

func (s *Service) scanPort(ctx context.Context, host string, port int, service string) PortResult {
	start := time.Now()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	conn, err := netguard.DialContext(dialCtx, "tcp", addr)
	cancel()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		status := "CLOSED"
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			status = "TIMEOUT"
		}
		return PortResult{
			Port:      port,
			Service:   service,
			IsOpen:    false,
			LatencyMs: latency,
			Status:    status,
		}
	}
	defer conn.Close()

	return PortResult{
		Port:      port,
		Service:   service,
		IsOpen:    true,
		LatencyMs: latency,
		Status:    "OPEN",
	}
}
