package instant

import (
	"context"
	"strings"
	"testing"
)

func TestTelegramOSINTDetection(t *testing.T) {
	ctx := context.Background()

	// 1. Test handle extraction
	testCases := []struct {
		query          string
		expectedHandle string
	}{
		{"tg: @durov", "durov"},
		{"tg durov", "durov"},
		{"telegram: @leakbase", "leakbase"},
		{"t.me/telegram", "telegram"},
		{"https://t.me/s/kominfo", "kominfo"},
		{"@cybersec_alerts", "cybersec_alerts"},
	}

	for _, tc := range testCases {
		ans := CheckTelegramOSINTQuery(ctx, tc.query)
		if ans == nil {
			t.Errorf("Expected InstantAnswer for query %q, got nil", tc.query)
			continue
		}
		if ans.Type != "telegram_osint" {
			t.Errorf("Expected Type 'telegram_osint', got %q", ans.Type)
		}
		if !strings.Contains(ans.Title, tc.expectedHandle) {
			t.Errorf("Expected Title to contain handle %q, got %q", tc.expectedHandle, ans.Title)
		}
	}
}

func TestCleanTelegramHandle(t *testing.T) {
	if cleanTelegramHandle("@durov") != "durov" {
		t.Errorf("Expected 'durov', got %q", cleanTelegramHandle("@durov"))
	}
	if cleanTelegramHandle("valid_channel_123") != "valid_channel_123" {
		t.Errorf("Expected 'valid_channel_123', got %q", cleanTelegramHandle("valid_channel_123"))
	}
	if cleanTelegramHandle("ab") != "" {
		t.Errorf("Expected short handle 'ab' to be rejected")
	}
	if cleanTelegramHandle("invalid handle with spaces") != "" {
		t.Errorf("Expected spaces to be rejected")
	}
}
