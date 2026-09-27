package threat

import (
	"context"
	"testing"
	"time"
)

func TestScanURLShortener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	report := ScanURL(ctx, "https://bit.ly/example-link")

	if !report.IsShortened {
		t.Errorf("expected bit.ly to be detected as a shortener")
	}

	found := false
	for _, s := range report.Signals {
		if s.Code == "URL_SHORTENER" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected URL_SHORTENER signal to be generated")
	}
}

func TestScanURLPunycode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	report := ScanURL(ctx, "https://xn--pple-43d.com/login")

	found := false
	for _, s := range report.Signals {
		if s.Code == "PUNYCODE_DOMAIN" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PUNYCODE_DOMAIN signal to be generated for xn-- domain")
	}
}

func TestScanURLRawIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	report := ScanURL(ctx, "http://192.168.1.1/admin")

	found := false
	for _, s := range report.Signals {
		if s.Code == "IP_URL" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected IP_URL signal to be generated for raw IP address")
	}
}

func TestScanURLSuspiciousTLD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	report := ScanURL(ctx, "http://secure-paypal-login.xyz/verify")

	if report.ThreatScore == 0 {
		t.Errorf("expected suspicious URL to have threat score > 0, got %d", report.ThreatScore)
	}

	foundTLD := false
	foundPhish := false
	for _, s := range report.Signals {
		if s.Code == "RISKY_TLD" {
			foundTLD = true
		}
		if s.Code == "BRAND_SPOOFING_KEYWORDS" {
			foundPhish = true
		}
	}
	if !foundTLD {
		t.Errorf("expected RISKY_TLD signal for .xyz")
	}
	if !foundPhish {
		t.Errorf("expected BRAND_SPOOFING_KEYWORDS signal for paypal/login/verify")
	}
}
