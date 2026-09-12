package instant

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// CheckBreachQuery checks if a query is requesting data leak verification or OSINT lookup
func CheckBreachQuery(ctx context.Context, query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. Password breach queries
	// Matches "pwned: password", "check password xyz", "leak: password", "hibp: pass"
	passPrefixes := []string{
		"pwned:", "check password ", "check pass ", "password leak ", "is pwned ",
		"pwned pass ", "check leaked password ", "hibp pass ", "leak pass ",
	}
	for _, p := range passPrefixes {
		if strings.HasPrefix(qLower, p) {
			target := strings.TrimSpace(q[len(p):])
			if target != "" {
				return checkPasswordBreach(ctx, target)
			}
		}
	}

	// 2. Email or Account breach lookup
	// Matches "breach: user@example.com", "leak: user@example.com", "hibp: user@example.com"
	accountPrefixes := []string{
		"breach:", "leak:", "hibp:", "pwned:", "check email ", "email leak ",
		"is leaked ", "data breach ", "breached ",
	}
	for _, p := range accountPrefixes {
		if strings.HasPrefix(qLower, p) {
			target := strings.TrimSpace(q[len(p):])
			if target != "" {
				return buildBreachReportCard(target)
			}
		}
	}

	// 3. Direct email address input
	if emailRegex.MatchString(q) {
		return buildBreachReportCard(q)
	}

	return nil
}

// checkPasswordBreach uses K-Anonymity SHA-1 prefix lookups (zero-knowledge) to check if password was leaked
func checkPasswordBreach(ctx context.Context, password string) *models.InstantAnswer {
	// 1. Calculate SHA-1 hash of password
	hasher := sha1.New()
	hasher.Write([]byte(password))
	hash := strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))

	if len(hash) < 5 {
		return nil
	}

	prefix := hash[:5]
	suffix := hash[5:]

	// 2. Request range from HaveIBeenPwned K-Anonymity API (only first 5 chars sent)
	apiURL := fmt.Sprintf("https://api.pwnedpasswords.com/range/%s", prefix)
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}

	req.Header.Set("User-Agent", "SearXGo-K-Anonymity-Checker/1.0")
	req.Header.Set("Add-Padding", "true") // Cloudflare padding against side-channel analysis

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// 3. Scan lines for remaining 35 characters
	scanner := bufio.NewScanner(resp.Body)
	leakCount := int64(0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			candidateSuffix := strings.ToUpper(strings.TrimSpace(parts[0]))
			if candidateSuffix == suffix {
				cnt, parseErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				if parseErr == nil {
					leakCount = cnt
					break
				}
			}
		}
	}

	if leakCount > 0 {
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "⚠️ Data Breach Alert: Password Compromised",
			Value:       fmt.Sprintf("Found %s times in known data breaches!", formatLeakNumber(leakCount)),
			Description: "This password has been exposed in publicly leaked databases and credentials dumps. Do NOT use this password anywhere.",
			URL:         "https://haveibeenpwned.com/Passwords",
			Attributes: map[string]string{
				"SHA-1 Prefix": prefix,
				"Status":       "COMPROMISED (High Risk)",
				"Privacy Mode": "K-Anonymity (Zero-Knowledge verification)",
			},
		}
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       "✅ Password Status: Not Leaked",
		Value:       "0 breaches found in public leak archives",
		Description: "This password does not appear in known public data breach dumps (verified via K-Anonymity SHA-1).",
		URL:         "https://haveibeenpwned.com/Passwords",
		Attributes: map[string]string{
			"SHA-1 Prefix": prefix,
			"Status":       "CLEAN",
			"Privacy Mode": "K-Anonymity (Zero-Knowledge verification)",
		},
	}
}

// buildBreachReportCard creates an OSINT investigation summary card for an email or domain
func buildBreachReportCard(target string) *models.InstantAnswer {
	escapedTarget := url.QueryEscape(target)
	pathEscaped := url.PathEscape(target)

	return &models.InstantAnswer{
		Type:        "infobox",
		Title:       fmt.Sprintf("🔍 Data Breach & OSINT Lookup: %s", target),
		Value:       "Public Security Disclosure & Leak Investigation",
		Description: fmt.Sprintf("Inspect compromise records, leaked databases, and text dumps for '%s' across HaveIBeenPwned, IntelX, Snusbase, and BreachDirectory.", target),
		URL:         fmt.Sprintf("https://haveibeenpwned.com/unifiedsearch/%s", pathEscaped),
		Attributes: map[string]string{
			"HaveIBeenPwned":  fmt.Sprintf("https://haveibeenpwned.com/account/%s", pathEscaped),
			"Intelligence X":  fmt.Sprintf("https://intelx.io/?s=%s", escapedTarget),
			"BreachDirectory": fmt.Sprintf("https://breachdirectory.org/?q=%s", escapedTarget),
			"Paste Dumps":     fmt.Sprintf("/search?q=%s+!pastes", escapedTarget),
		},
	}
}

func formatLeakNumber(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if numOfDigits <= 3 {
		return in
	}
	var out []byte
	rem := numOfDigits % 3
	if rem > 0 {
		out = append(out, in[:rem]...)
		if numOfDigits > 3 {
			out = append(out, ',')
		}
	}
	for i := rem; i < numOfDigits; i += 3 {
		out = append(out, in[i:i+3]...)
		if i+3 < numOfDigits {
			out = append(out, ',')
		}
	}
	return string(out)
}
