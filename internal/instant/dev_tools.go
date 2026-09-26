package instant

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

// CheckDevTools evaluates instant queries for developers: JWT, Cron, Chmod, and Regex
func CheckDevTools(query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. JWT Inspector: "jwt eyJ...", "jwt: eyJ...", or raw JWT token
	if strings.HasPrefix(qLower, "jwt ") || strings.HasPrefix(qLower, "jwt:") || looksLikeJWT(q) {
		token := q
		if strings.HasPrefix(qLower, "jwt:") {
			token = strings.TrimSpace(q[4:])
		} else if strings.HasPrefix(qLower, "jwt ") {
			token = strings.TrimSpace(q[4:])
		}
		if ans := inspectJWT(token); ans != nil {
			return ans
		}
	}

	// 2. Cron Expression Explainer: "cron */15 * * * *", "crontab 0 0 * * 1"
	if strings.HasPrefix(qLower, "cron ") || strings.HasPrefix(qLower, "cron:") || strings.HasPrefix(qLower, "crontab ") {
		expr := q
		if strings.HasPrefix(qLower, "cron:") {
			expr = strings.TrimSpace(q[5:])
		} else if strings.HasPrefix(qLower, "cron ") {
			expr = strings.TrimSpace(q[5:])
		} else if strings.HasPrefix(qLower, "crontab ") {
			expr = strings.TrimSpace(q[8:])
		}
		if ans := explainCron(expr); ans != nil {
			return ans
		}
	}

	// 3. Chmod Permission Calculator: "chmod 755", "chmod: 644", "chmod rwxr-xr-x"
	if strings.HasPrefix(qLower, "chmod ") || strings.HasPrefix(qLower, "chmod:") {
		arg := q
		if strings.HasPrefix(qLower, "chmod:") {
			arg = strings.TrimSpace(q[6:])
		} else {
			arg = strings.TrimSpace(q[6:])
		}
		if ans := calculateChmod(arg); ans != nil {
			return ans
		}
	}

	// 4. Regex Pattern Validator & Explainer: "regex ^[a-z0-9]+$", "regexp \d{3}-\d{4}"
	if strings.HasPrefix(qLower, "regex ") || strings.HasPrefix(qLower, "regex:") || strings.HasPrefix(qLower, "regexp ") {
		pattern := q
		if strings.HasPrefix(qLower, "regex:") {
			pattern = strings.TrimSpace(q[6:])
		} else if strings.HasPrefix(qLower, "regex ") {
			pattern = strings.TrimSpace(q[6:])
		} else if strings.HasPrefix(qLower, "regexp ") {
			pattern = strings.TrimSpace(q[7:])
		}
		if ans := explainRegex(pattern); ans != nil {
			return ans
		}
	}

	// 5. Domain Recon & Auditor trigger: "recon example.com", "recon: example.com", "audit github.com"
	if strings.HasPrefix(qLower, "recon ") || strings.HasPrefix(qLower, "recon:") || strings.HasPrefix(qLower, "audit ") {
		var target string
		if strings.HasPrefix(qLower, "recon:") {
			target = strings.TrimSpace(q[6:])
		} else if strings.HasPrefix(qLower, "recon ") {
			target = strings.TrimSpace(q[6:])
		} else if strings.HasPrefix(qLower, "audit ") {
			target = strings.TrimSpace(q[6:])
		}
		if target != "" {
			cleanTarget := strings.TrimPrefix(target, "https://")
			cleanTarget = strings.TrimPrefix(cleanTarget, "http://")
			cleanTarget = strings.TrimSpace(cleanTarget)
			return &models.InstantAnswer{
				Type:        "tools",
				Title:       "🔍 Domain Security Recon: " + cleanTarget,
				Value:       "Audit SSL/TLS, HTTP Security Headers, DNS, and Tech Stack",
				Description: fmt.Sprintf("<a href=\"/recon?domain=%s\" class=\"action-btn btn-searx\" style=\"display:inline-flex; align-items:center; gap:0.4rem; background:var(--accent-gradient); color:#fff; padding:0.45rem 1rem; border-radius:6px; text-decoration:none; font-weight:700; font-size:0.85rem; margin-top:0.5rem;\">🚀 Launch Deep Recon for %s &raquo;</a>", url.QueryEscape(cleanTarget), cleanTarget),
			}
		}
	}

	return nil
}

func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	return strings.HasPrefix(parts[0], "eyJ")
}

func inspectJWT(token string) *models.InstantAnswer {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil
	}

	decodePart := func(seg string) ([]byte, error) {
		// Base64Url padding
		switch len(seg) % 4 {
		case 2:
			seg += "=="
		case 3:
			seg += "="
		}
		return base64.URLEncoding.DecodeString(seg)
	}

	headerBytes, err1 := decodePart(parts[0])
	payloadBytes, err2 := decodePart(parts[1])
	if err1 != nil || err2 != nil {
		return nil
	}

	var headerMap map[string]interface{}
	var payloadMap map[string]interface{}
	_ = json.Unmarshal(headerBytes, &headerMap)
	_ = json.Unmarshal(payloadBytes, &payloadMap)

	attrs := make(map[string]string)
	if alg, ok := headerMap["alg"].(string); ok {
		attrs["Algorithm"] = alg
	}
	if typ, ok := headerMap["typ"].(string); ok {
		attrs["Type"] = typ
	}

	status := "Valid Structure"
	var expiryInfo string

	// Check Expiration
	if expVal, exists := payloadMap["exp"]; exists {
		var expUnix int64
		switch v := expVal.(type) {
		case float64:
			expUnix = int64(v)
		case int64:
			expUnix = v
		}
		if expUnix > 0 {
			expTime := time.Unix(expUnix, 0).UTC()
			now := time.Now().UTC()
			if now.After(expTime) {
				status = "🔴 Token Expired"
				expiryInfo = fmt.Sprintf("Expired on %s (%s ago)", expTime.Format("2006-01-02 15:04:05 UTC"), now.Sub(expTime).Round(time.Minute))
			} else {
				status = "🟢 Token Active (Not Expired)"
				expiryInfo = fmt.Sprintf("Expires on %s (in %s)", expTime.Format("2006-01-02 15:04:05 UTC"), expTime.Sub(now).Round(time.Minute))
			}
			attrs["Expiry Status"] = expiryInfo
		}
	}

	if sub, ok := payloadMap["sub"].(string); ok {
		attrs["Subject (sub)"] = sub
	}
	if iss, ok := payloadMap["iss"].(string); ok {
		attrs["Issuer (iss)"] = iss
	}
	if aud, ok := payloadMap["aud"].(string); ok {
		attrs["Audience (aud)"] = aud
	}

	var prettyPayload bytes.Buffer
	_ = json.Indent(&prettyPayload, payloadBytes, "", "  ")

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       "🔑 JWT Token Inspector",
		Value:       status,
		Description: fmt.Sprintf("Decoded Payload:\n%s", prettyPayload.String()),
		URL:         "https://jwt.io",
		Attributes:  attrs,
	}
}

func explainCron(expr string) *models.InstantAnswer {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	humanText := describeCronSchedule(minute, hour, dom, month, dow)
	attrs := map[string]string{
		"1. Minute":       minute,
		"2. Hour":         hour,
		"3. Day of Month": dom,
		"4. Month":        month,
		"5. Day of Week":  dow,
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("⏰ Cron Expression: %s", expr),
		Value:       humanText,
		Description: fmt.Sprintf("Standard 5-part cron schedule breakdown for '%s'.", expr),
		URL:         fmt.Sprintf("https://crontab.guru/#%s", strings.ReplaceAll(expr, " ", "_")),
		Attributes:  attrs,
	}
}

func describeCronSchedule(min, hr, dom, mon, dow string) string {
	var parts []string

	// Minute
	if min == "*" {
		parts = append(parts, "Every minute")
	} else if strings.HasPrefix(min, "*/") {
		parts = append(parts, fmt.Sprintf("Every %s minutes", min[2:]))
	} else {
		parts = append(parts, fmt.Sprintf("At minute %s", min))
	}

	// Hour
	if hr == "*" {
		if !strings.HasPrefix(min, "*/") && min != "*" {
			parts = append(parts, "of every hour")
		}
	} else if strings.HasPrefix(hr, "*/") {
		parts = append(parts, fmt.Sprintf("past every %s hours", hr[2:]))
	} else {
		parts = append(parts, fmt.Sprintf("past hour %s:00", hr))
	}

	// Day of Month / Week
	if dow != "*" {
		parts = append(parts, fmt.Sprintf("on weekday (%s)", dow))
	}
	if dom != "*" {
		parts = append(parts, fmt.Sprintf("on day %s of the month", dom))
	}
	if mon != "*" {
		parts = append(parts, fmt.Sprintf("in month %s", mon))
	}

	return strings.Join(parts, ", ")
}

func calculateChmod(val string) *models.InstantAnswer {
	clean := strings.TrimSpace(val)

	// Check if Octal (e.g. 755, 644, 0755)
	clean = strings.TrimPrefix(clean, "0")
	if len(clean) == 3 {
		if u, err := strconv.Atoi(string(clean[0])); err == nil && u >= 0 && u <= 7 {
			if g, err := strconv.Atoi(string(clean[1])); err == nil && g >= 0 && g <= 7 {
				if o, err := strconv.Atoi(string(clean[2])); err == nil && o >= 0 && o <= 7 {
					permToSym := func(n int) string {
						r := (n & 4) != 0
						w := (n & 2) != 0
						x := (n & 1) != 0
						s := ""
						if r { s += "r" } else { s += "-" }
						if w { s += "w" } else { s += "-" }
						if x { s += "x" } else { s += "-" }
						return s
					}

					permToDesc := func(n int) string {
						var d []string
						if (n & 4) != 0 { d = append(d, "Read") }
						if (n & 2) != 0 { d = append(d, "Write") }
						if (n & 1) != 0 { d = append(d, "Execute") }
						if len(d) == 0 { return "No Permissions (---)" }
						return strings.Join(d, " + ")
					}

					symStr := fmt.Sprintf("-%s%s%s", permToSym(u), permToSym(g), permToSym(o))
					attrs := map[string]string{
						"User (Owner)": fmt.Sprintf("%d (%s) -> %s", u, permToSym(u), permToDesc(u)),
						"Group":        fmt.Sprintf("%d (%s) -> %s", g, permToSym(g), permToDesc(g)),
						"Others":       fmt.Sprintf("%d (%s) -> %s", o, permToSym(o), permToDesc(o)),
						"Command":      fmt.Sprintf("chmod %s filename", clean),
					}

					return &models.InstantAnswer{
						Type:        "tools",
						Title:       fmt.Sprintf("🛡️ Linux File Permissions: chmod %s", clean),
						Value:       symStr,
						Description: fmt.Sprintf("Octal %s maps to symbolic notation %s.", clean, symStr),
						URL:         fmt.Sprintf("https://chmod-calculator.com/?mode=%s", clean),
						Attributes:  attrs,
					}
				}
			}
		}
	}

	return nil
}

func explainRegex(pattern string) *models.InstantAnswer {
	if len(pattern) == 0 {
		return nil
	}

	_, err := regexp.Compile(pattern)
	status := "🟢 Valid Regular Expression Pattern"
	errMsg := ""
	if err != nil {
		status = "🔴 Invalid Regex Syntax"
		errMsg = err.Error()
	}

	attrs := make(map[string]string)
	attrs["Pattern"] = pattern
	if errMsg != "" {
		attrs["Error Details"] = errMsg
	} else {
		var elements []string
		if strings.HasPrefix(pattern, "^") { elements = append(elements, "^: Anchors to beginning of string") }
		if strings.HasSuffix(pattern, "$") { elements = append(elements, "$: Anchors to end of string") }
		if strings.Contains(pattern, `\d`) { elements = append(elements, `\d: Matches any digit (0-9)`) }
		if strings.Contains(pattern, `\w`) { elements = append(elements, `\w: Matches any word character`) }
		if strings.Contains(pattern, `\s`) { elements = append(elements, `\s: Matches any whitespace character`) }
		if strings.Contains(pattern, "+") { elements = append(elements, "+: Matches 1 or more occurrences") }
		if strings.Contains(pattern, "*") { elements = append(elements, "*: Matches 0 or more occurrences") }
		if strings.Contains(pattern, "?") { elements = append(elements, "?: Matches 0 or 1 occurrence") }

		if len(elements) > 0 {
			attrs["Syntax Elements"] = strings.Join(elements, " | ")
		}
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🧪 Regular Expression: %s", pattern),
		Value:       status,
		Description: fmt.Sprintf("Go regexp compatibility evaluation for '%s'.", pattern),
		URL:         fmt.Sprintf("https://regex101.com/?regex=%s", strings.ReplaceAll(pattern, " ", "+")),
		Attributes:  attrs,
	}
}
