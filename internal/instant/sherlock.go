package instant

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"searxgo/internal/models"
)

type SherlockPlatform struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Icon       string `json:"icon"`
	CheckURL   string `json:"check_url"`
	ProfileURL string `json:"profile_url"`
	Method     string `json:"method"`
}

type SherlockProfile struct {
	Platform   string `json:"platform"`
	Category   string `json:"category"`
	Icon       string `json:"icon"`
	URL        string `json:"url"`
	Found      bool   `json:"found"`
	StatusCode int    `json:"status_code,omitempty"`
}

type SherlockReport struct {
	Username     string            `json:"username"`
	Total        int               `json:"total"`
	FoundCount   int               `json:"found_count"`
	ScanDuration string            `json:"scan_duration"`
	Results      []SherlockProfile `json:"results"`
}

var rxUsername = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{3,32}$`)

// CleanUsername checks and normalizes a username
func CleanUsername(u string) (string, bool) {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(u, "@")
	if !rxUsername.MatchString(u) {
		return "", false
	}
	return u, true
}

// CheckSherlockQuery checks if the query is a username OSINT lookup
func CheckSherlockQuery(ctx context.Context, query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	prefixes := []string{"user:", "username:", "sherlock:", "osint:", "handle:"}
	targetUser := ""
	for _, p := range prefixes {
		if strings.HasPrefix(qLower, p) {
			targetUser = strings.TrimSpace(q[len(p):])
			break
		}
	}

	if targetUser == "" {
		return nil
	}

	cleanUser, ok := CleanUsername(targetUser)
	if !ok {
		return nil
	}

	return runSherlockScan(ctx, cleanUser)
}

// GetSherlockPlatforms returns the catalog of supported platforms
func GetSherlockPlatforms(username string) []SherlockPlatform {
	return []SherlockPlatform{
		// Developer & Code
		{
			Name:       "GitHub",
			Category:   "dev",
			Icon:       "💻",
			CheckURL:   fmt.Sprintf("https://api.github.com/users/%s", username),
			ProfileURL: fmt.Sprintf("https://github.com/%s", username),
			Method:     "GET",
		},
		{
			Name:       "GitLab",
			Category:   "dev",
			Icon:       "🦊",
			CheckURL:   fmt.Sprintf("https://gitlab.com/%s", username),
			ProfileURL: fmt.Sprintf("https://gitlab.com/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Codeberg",
			Category:   "dev",
			Icon:       "🏔️",
			CheckURL:   fmt.Sprintf("https://codeberg.org/%s", username),
			ProfileURL: fmt.Sprintf("https://codeberg.org/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "DockerHub",
			Category:   "dev",
			Icon:       "🐳",
			CheckURL:   fmt.Sprintf("https://hub.docker.com/v2/users/%s/", username),
			ProfileURL: fmt.Sprintf("https://hub.docker.com/u/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Dev.to",
			Category:   "dev",
			Icon:       "👩‍💻",
			CheckURL:   fmt.Sprintf("https://dev.to/%s", username),
			ProfileURL: fmt.Sprintf("https://dev.to/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "NPM",
			Category:   "dev",
			Icon:       "📦",
			CheckURL:   fmt.Sprintf("https://www.npmjs.com/~%s", username),
			ProfileURL: fmt.Sprintf("https://www.npmjs.com/~%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "PyPI",
			Category:   "dev",
			Icon:       "🐍",
			CheckURL:   fmt.Sprintf("https://pypi.org/user/%s/", username),
			ProfileURL: fmt.Sprintf("https://pypi.org/user/%s/", username),
			Method:     "HEAD",
		},
		{
			Name:       "HackerNews",
			Category:   "dev",
			Icon:       "🟠",
			CheckURL:   fmt.Sprintf("https://news.ycombinator.com/user?id=%s", username),
			ProfileURL: fmt.Sprintf("https://news.ycombinator.com/user?id=%s", username),
			Method:     "GET",
		},

		// Social & Discussion
		{
			Name:       "Reddit",
			Category:   "social",
			Icon:       "👾",
			CheckURL:   fmt.Sprintf("https://www.reddit.com/user/%s/about.json", username),
			ProfileURL: fmt.Sprintf("https://www.reddit.com/user/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Telegram",
			Category:   "social",
			Icon:       "✈️",
			CheckURL:   fmt.Sprintf("https://t.me/%s", username),
			ProfileURL: fmt.Sprintf("https://t.me/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Medium",
			Category:   "social",
			Icon:       "✍️",
			CheckURL:   fmt.Sprintf("https://medium.com/@%s", username),
			ProfileURL: fmt.Sprintf("https://medium.com/@%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Keybase",
			Category:   "social",
			Icon:       "🗝️",
			CheckURL:   fmt.Sprintf("https://keybase.io/%s", username),
			ProfileURL: fmt.Sprintf("https://keybase.io/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Mastodon",
			Category:   "social",
			Icon:       "🐘",
			CheckURL:   fmt.Sprintf("https://mastodon.social/@%s.json", username),
			ProfileURL: fmt.Sprintf("https://mastodon.social/@%s", username),
			Method:     "GET",
		},
		{
			Name:       "Disqus",
			Category:   "social",
			Icon:       "💬",
			CheckURL:   fmt.Sprintf("https://disqus.com/by/%s/", username),
			ProfileURL: fmt.Sprintf("https://disqus.com/by/%s/", username),
			Method:     "HEAD",
		},

		// Gaming
		{
			Name:       "Steam",
			Category:   "gaming",
			Icon:       "🎮",
			CheckURL:   fmt.Sprintf("https://steamcommunity.com/id/%s", username),
			ProfileURL: fmt.Sprintf("https://steamcommunity.com/id/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Chess.com",
			Category:   "gaming",
			Icon:       "♟️",
			CheckURL:   fmt.Sprintf("https://api.chess.com/pub/player/%s", username),
			ProfileURL: fmt.Sprintf("https://www.chess.com/member/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Twitch",
			Category:   "gaming",
			Icon:       "🟣",
			CheckURL:   fmt.Sprintf("https://www.twitch.tv/%s", username),
			ProfileURL: fmt.Sprintf("https://www.twitch.tv/%s", username),
			Method:     "HEAD",
		},

		// Creators & Media
		{
			Name:       "SoundCloud",
			Category:   "creators",
			Icon:       "🎵",
			CheckURL:   fmt.Sprintf("https://soundcloud.com/%s", username),
			ProfileURL: fmt.Sprintf("https://soundcloud.com/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Vimeo",
			Category:   "creators",
			Icon:       "▶️",
			CheckURL:   fmt.Sprintf("https://vimeo.com/%s", username),
			ProfileURL: fmt.Sprintf("https://vimeo.com/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "BuyMeACoffee",
			Category:   "creators",
			Icon:       "☕",
			CheckURL:   fmt.Sprintf("https://buymeacoffee.com/%s", username),
			ProfileURL: fmt.Sprintf("https://buymeacoffee.com/%s", username),
			Method:     "HEAD",
		},
	}
}

// ScanUsernameProfiles scans all platforms concurrently for the target username
func ScanUsernameProfiles(parentCtx context.Context, username string, timeout time.Duration) *SherlockReport {
	startTime := time.Now()
	cleanUser, ok := CleanUsername(username)
	if !ok {
		return &SherlockReport{
			Username:     username,
			Total:        0,
			FoundCount:   0,
			ScanDuration: "0ms",
			Results:      nil,
		}
	}

	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	platforms := GetSherlockPlatforms(cleanUser)
	client := &http.Client{
		Timeout: 2000 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	var wg sync.WaitGroup
	type tempRes struct {
		idx  int
		prof SherlockProfile
	}
	ch := make(chan tempRes, len(platforms))

	for i, p := range platforms {
		wg.Add(1)
		go func(idx int, plat SherlockPlatform) {
			defer wg.Done()
			prof := SherlockProfile{
				Platform: plat.Name,
				Category: plat.Category,
				Icon:     plat.Icon,
				URL:      plat.ProfileURL,
				Found:    false,
			}

			req, err := http.NewRequestWithContext(ctx, plat.Method, plat.CheckURL, nil)
			if err != nil {
				ch <- tempRes{idx: idx, prof: prof}
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "*/*")

			resp, err := client.Do(req)
			if err != nil {
				ch <- tempRes{idx: idx, prof: prof}
				return
			}
			defer resp.Body.Close()

			prof.StatusCode = resp.StatusCode
			if resp.StatusCode == 200 {
				prof.Found = true
			}

			ch <- tempRes{idx: idx, prof: prof}
		}(i, p)
	}

	wg.Wait()
	close(ch)

	results := make([]SherlockProfile, len(platforms))
	foundCount := 0
	for r := range ch {
		results[r.idx] = r.prof
		if r.prof.Found {
			foundCount++
		}
	}

	return &SherlockReport{
		Username:     cleanUser,
		Total:        len(platforms),
		FoundCount:   foundCount,
		ScanDuration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
		Results:      results,
	}
}

func runSherlockScan(parentCtx context.Context, username string) *models.InstantAnswer {
	report := ScanUsernameProfiles(parentCtx, username, 2200*time.Millisecond)

	var foundList []SherlockProfile
	var notFoundList []SherlockProfile

	for _, res := range report.Results {
		if res.Found {
			foundList = append(foundList, res)
		} else {
			notFoundList = append(notFoundList, res)
		}
	}

	// Format results into modern HTML cards
	var foundHTML strings.Builder
	for _, item := range foundList {
		foundHTML.WriteString(fmt.Sprintf(`
		<a href="%s" target="_blank" rel="noreferrer noopener" style="display:inline-flex; align-items:center; gap:0.4rem; padding:0.4rem 0.75rem; background:rgba(16, 185, 129, 0.12); border:1px solid rgba(16, 185, 129, 0.4); border-radius:var(--radius-sm); text-decoration:none; color:var(--text-primary); font-size:0.85rem; font-weight:600; transition:transform 0.15s ease;" onmouseover="this.style.transform='translateY(-2px)'" onmouseout="this.style.transform='translateY(0)'">
			<span>%s %s</span>
			<span style="color:var(--accent-emerald); font-size:0.75rem;">✓ Found ➔</span>
		</a>`, item.URL, item.Icon, item.Platform))
	}

	var missingHTML strings.Builder
	for _, item := range notFoundList {
		missingHTML.WriteString(fmt.Sprintf(`
		<span style="display:inline-flex; align-items:center; gap:0.3rem; padding:0.35rem 0.6rem; background:var(--bg-glass-card); border:1px solid var(--border-glass); border-radius:var(--radius-sm); color:var(--text-muted); font-size:0.8rem;">
			<span>%s %s</span>
		</span>`, item.Icon, item.Platform))
	}

	desc := fmt.Sprintf(`
	<div style="margin-top:0.75rem;">
		<div style="font-size:0.8rem; text-transform:uppercase; letter-spacing:0.05em; color:var(--accent-emerald); font-weight:700; margin-bottom:0.5rem;">
			Active Profiles (%d detected):
		</div>
		<div style="display:flex; flex-wrap:wrap; gap:0.5rem; margin-bottom:1rem;">
			%s
		</div>

		<div style="font-size:0.75rem; text-transform:uppercase; letter-spacing:0.05em; color:var(--text-muted); font-weight:600; margin-bottom:0.4rem;">
			Other Platforms Checked (%d):
		</div>
		<div style="display:flex; flex-wrap:wrap; gap:0.4rem; margin-bottom:0.75rem;">
			%s
		</div>

		<div style="border-top:1px solid var(--border-glass); padding-top:0.5rem; font-size:0.8rem; color:var(--text-secondary);">
			<a href="/sherlock?username=%s" style="color:var(--accent-cyan); font-weight:600; text-decoration:none;">🚀 Open Dedicated OSINT Sherlock Workspace ➔</a>
		</div>
	</div>`, len(foundList), foundHTML.String(), len(notFoundList), missingHTML.String(), username)

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🕵️ OSINT Social Username Recon: @%s", username),
		Value:       fmt.Sprintf("%d active accounts located across %d platforms (%s)", len(foundList), report.Total, report.ScanDuration),
		Description: desc,
		URL:         fmt.Sprintf("/sherlock?username=%s", username),
	}
}
