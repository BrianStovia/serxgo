package ipintel

import (
	"context"
	"testing"
	"time"
)

func TestIPIntelPrivate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	report := Investigate(ctx, "127.0.0.1")
	if !report.IsPrivate {
		t.Errorf("expected 127.0.0.1 to be flagged as private")
	}
	if report.IP != "127.0.0.1" {
		t.Errorf("expected IP to be 127.0.0.1, got %s", report.IP)
	}

	reportRFC := Investigate(ctx, "192.168.1.100")
	if !reportRFC.IsPrivate {
		t.Errorf("expected 192.168.1.100 to be private")
	}
}

func TestIPIntelEmpty(t *testing.T) {
	ctx := context.Background()
	report := Investigate(ctx, "")
	if report.Error == "" {
		t.Errorf("expected error for empty target")
	}
}
