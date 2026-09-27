package instant

import (
	"context"
	"strings"
	"testing"
)

func TestTimezoneConverter(t *testing.T) {
	queries := []string{
		"time in tokyo",
		"time in london",
		"time in jakarta",
		"10am utc to wib",
		"14:00 est to pst",
	}

	for _, q := range queries {
		ans := CheckConverters(q)
		if ans == nil {
			t.Errorf("expected timezone answer for %q, got nil", q)
		} else if ans.Value == "" {
			t.Errorf("expected non-empty value for %q", q)
		}
	}
}

func TestStorageConverter(t *testing.T) {
	queries := []string{
		"500 gb to tb",
		"1024 mb in gb",
		"2 tb to gb",
	}

	for _, q := range queries {
		ans := CheckConverters(q)
		if ans == nil {
			t.Errorf("expected storage answer for %q, got nil", q)
		} else if !strings.Contains(ans.Value, "=") {
			t.Errorf("expected '=' in answer for %q, got %q", q, ans.Value)
		}
	}
}

func TestColorTool(t *testing.T) {
	queries := []string{
		"#ff5722",
		"rgb(255, 87, 34)",
		"hsl(14, 100%, 57%)",
		"color coral",
	}

	for _, q := range queries {
		ans := CheckColorTool(q)
		if ans == nil {
			t.Errorf("expected color answer for %q, got nil", q)
		} else if !strings.Contains(ans.Description, "HEX") {
			t.Errorf("expected HEX breakdown in description for %q", q)
		}
	}
}

func TestSecretGen(t *testing.T) {
	queries := []string{
		"secret",
		"token 32",
		"generate api key",
		"nanoid",
	}

	for _, q := range queries {
		ans := CheckSecretGen(q)
		if ans == nil {
			t.Errorf("expected secret generator answer for %q, got nil", q)
		} else if len(ans.Value) < 8 {
			t.Errorf("expected secret value >= 8 chars for %q, got %q", q, ans.Value)
		}
	}
}

func TestSherlockUsernameQuery(t *testing.T) {
	ctx := context.Background()
	ans := CheckSherlockQuery(ctx, "user: torvalds")
	if ans == nil {
		t.Fatalf("expected sherlock answer for user: torvalds, got nil")
	}
	if !strings.Contains(ans.Title, "torvalds") {
		t.Errorf("expected title to mention username, got %q", ans.Title)
	}
}
