package instant

import (
	"context"
	"strings"
	"testing"
)

func TestTools(t *testing.T) {
	ctx := context.Background()

	// 1. IP
	ipAns := CheckInstantTools(ctx, "what is my ip", "192.168.1.50", "TestAgent")
	if ipAns == nil || ipAns.Value != "192.168.1.50" {
		t.Errorf("IP test failed, got: %+v", ipAns)
	}

	// 2. User-Agent
	uaAns := CheckInstantTools(ctx, "my user agent", "192.168.1.50", "TestBrowser/1.0")
	if uaAns == nil || uaAns.Value != "TestBrowser/1.0" {
		t.Errorf("UA test failed, got: %+v", uaAns)
	}

	// 3. Tor Check
	torAns := CheckInstantTools(ctx, "tor check", "192.168.1.50", "TestAgent")
	if torAns == nil || !strings.Contains(torAns.Value, "Not Using Tor") {
		t.Errorf("Tor check failed, got: %+v", torAns)
	}

	// 4. Hashes
	shaAns := CheckInstantTools(ctx, "sha256 hello", "", "")
	if shaAns == nil || shaAns.Value != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Errorf("SHA256 test failed, got: %+v", shaAns)
	}

	md5Ans := CheckInstantTools(ctx, "md5 hello", "", "")
	if md5Ans == nil || md5Ans.Value != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("MD5 test failed, got: %+v", md5Ans)
	}

	// 5. UUID
	uuidAns := CheckInstantTools(ctx, "uuid", "", "")
	if uuidAns == nil || len(uuidAns.Value) != 36 {
		t.Errorf("UUID test failed, got: %+v", uuidAns)
	}

	// 6. ROT13
	rotAns := CheckInstantTools(ctx, "rot13 hello", "", "")
	if rotAns == nil || rotAns.Value != "uryyb" {
		t.Errorf("ROT13 test failed, got: %+v", rotAns)
	}

	// 7. Reverse
	revAns := CheckInstantTools(ctx, "reverse hello", "", "")
	if revAns == nil || revAns.Value != "olleh" {
		t.Errorf("Reverse test failed, got: %+v", revAns)
	}
}

func TestBreachInstant(t *testing.T) {
	ctx := context.Background()

	// 1. Email query card
	ansEmail := CheckBreachQuery(ctx, "breach: test@example.com")
	if ansEmail == nil || !strings.Contains(ansEmail.Title, "test@example.com") {
		t.Fatalf("Expected breach card for test@example.com, got: %+v", ansEmail)
	}

	// 2. Direct email input
	ansDirect := CheckBreachQuery(ctx, "security-alert@company.org")
	if ansDirect == nil || !strings.Contains(ansDirect.Title, "security-alert@company.org") {
		t.Fatalf("Expected breach card for security-alert@company.org, got: %+v", ansDirect)
	}

	// 3. Format leak number
	numStr := formatLeakNumber(1234567)
	if numStr != "1,234,567" {
		t.Fatalf("Expected 1,234,567, got %s", numStr)
	}
}

