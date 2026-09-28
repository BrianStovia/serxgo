package cve

import (
	"context"
	"testing"
)

func TestCVEGetDefault(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	res := svc.GetCVEs(ctx, "", "ALL", false)
	if res.Total == 0 {
		t.Fatalf("expected seeded CVE items, got 0")
	}

	// Filter by keyword
	resSSH := svc.GetCVEs(ctx, "OpenSSH", "ALL", false)
	if resSSH.Total == 0 {
		t.Errorf("expected OpenSSH CVEs, got 0")
	}

	// Filter by severity
	resCrit := svc.GetCVEs(ctx, "", "CRITICAL", false)
	for _, it := range resCrit.Items {
		if it.Severity != "CRITICAL" {
			t.Errorf("expected CRITICAL severity, got %s", it.Severity)
		}
	}

	// Filter by KEV
	resKEV := svc.GetCVEs(ctx, "", "ALL", true)
	for _, it := range resKEV.Items {
		if !it.IsKEV {
			t.Errorf("expected KEV item, got false")
		}
	}
}
