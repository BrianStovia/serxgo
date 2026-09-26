package instant

import (
	"strings"
	"testing"
)

func TestDevToolsChmod(t *testing.T) {
	ans := CheckDevTools("chmod 755")
	if ans == nil {
		t.Fatalf("Expected chmod 755 answer, got nil")
	}
	if !strings.Contains(ans.Value, "-rwxr-xr-x") {
		t.Errorf("Expected -rwxr-xr-x, got: %s", ans.Value)
	}

	ans644 := CheckDevTools("chmod: 644")
	if ans644 == nil || !strings.Contains(ans644.Value, "-rw-r--r--") {
		t.Errorf("Expected -rw-r--r--, got: %+v", ans644)
	}
}

func TestDevToolsCron(t *testing.T) {
	ans := CheckDevTools("cron */15 * * * *")
	if ans == nil {
		t.Fatalf("Expected cron answer, got nil")
	}
	if !strings.Contains(ans.Value, "Every 15 minutes") {
		t.Errorf("Expected 'Every 15 minutes', got: %s", ans.Value)
	}
}

func TestDevToolsRegex(t *testing.T) {
	ans := CheckDevTools(`regex ^[a-zA-Z0-9_]+$`)
	if ans == nil {
		t.Fatalf("Expected regex answer, got nil")
	}
	if !strings.Contains(ans.Value, "Valid") {
		t.Errorf("Expected valid regex, got: %s", ans.Value)
	}

	invalidAns := CheckDevTools(`regex [a-z`)
	if invalidAns == nil || !strings.Contains(invalidAns.Value, "Invalid") {
		t.Errorf("Expected invalid regex warning, got: %+v", invalidAns)
	}
}

func TestDevToolsJWT(t *testing.T) {
	// Sample dummy JWT: {"alg":"HS256","typ":"JWT"}.{"sub":"1234567890","name":"John Doe","iat":1516239022}.signature
	dummyToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	ans := CheckDevTools("jwt " + dummyToken)
	if ans == nil {
		t.Fatalf("Expected JWT answer, got nil")
	}
	if !strings.Contains(ans.Attributes["Algorithm"], "HS256") {
		t.Errorf("Expected HS256 algorithm, got: %+v", ans.Attributes)
	}
}
