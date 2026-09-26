package recon

import (
	"testing"
)

func TestCleanDomain(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://example.com/path/to/page", "example.com"},
		{"http://sub.domain.org:8080/test", "sub.domain.org"},
		{"google.com", "google.com"},
		{"  HTTPS://MY-SITE.IO/  ", "my-site.io"},
	}

	for _, c := range cases {
		got := CleanDomain(c.input)
		if got != c.expected {
			t.Errorf("CleanDomain(%s) = %s, expected %s", c.input, got, c.expected)
		}
	}
}
