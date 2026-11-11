// Package netguard provides outbound HTTP helpers for user supplied URLs.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// IsPublicIP reports whether ip is a globally routable unicast address.
func IsPublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// ValidatePublicURL accepts only absolute HTTP(S) URLs whose resolved addresses
// are all public. Rejecting mixed DNS answers prevents selection of a private
// address by a later resolver lookup.
func ValidatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("URL must be an absolute HTTP or HTTPS URL without credentials")
	}
	if port := u.Port(); port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return nil, fmt.Errorf("invalid URL port")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := lookupPublicIPs(ctx, u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

func lookupPublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if !IsPublicIP(ip) {
			return nil, fmt.Errorf("requests to private or non-public network addresses are restricted")
		}
		return []net.IP{ip}, nil
	}
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return nil, fmt.Errorf("requests to localhost are restricted")
	}
	answers, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(answers) == 0 {
		return nil, fmt.Errorf("could not resolve public target host")
	}
	ips := make([]net.IP, 0, len(answers))
	for _, answer := range answers {
		if !IsPublicIP(answer.IP) {
			return nil, fmt.Errorf("target host resolves to a private or non-public network address")
		}
		ips = append(ips, answer.IP)
	}
	return ips, nil
}

// DialContext resolves the host at connection time and dials the validated IP
// directly, preventing DNS rebinding between validation and connection.
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := lookupPublicIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// NewHTTPClient returns a client that blocks private targets and validates
// redirects. It deliberately does not use environment-configured HTTP proxies.
func NewHTTPClient(timeout time.Duration, maxRedirects int) *http.Client {
	transport := &http.Transport{
		DialContext:           DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   8,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if maxRedirects >= 0 && len(via) > maxRedirects {
				return http.ErrUseLastResponse
			}
			_, err := ValidatePublicURL(req.URL.String())
			return err
		},
	}
}
