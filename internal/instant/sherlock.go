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

type socialPlatform struct {
	Name    string
	Icon    string
	CheckURL string
	ProfileURL string
	Method  string
	CheckFunc func(resp *http.Response) bool
}

var rxUsername = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{3,32}$`)

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

	// Validate username format
	targetUser = strings.TrimPrefix(targetUser, "@")
	if !rxUsername.MatchString(targetUser) {
		return nil
	}

	return runSherlockScan(ctx, targetUser)
}

type scanResult struct {
	Platform socialPlatform
	Found    bool
	URL      string
}

func runSherlockScan(parentCtx context.Context, username string) *models.InstantAnswer {
	ctx, cancel := context.WithTimeout(parentCtx, 2200*time.Millisecond)
	defer cancel()

	platforms := []socialPlatform{
		{
			Name:       "GitHub",
			Icon:       "💻",
			CheckURL:   fmt.Sprintf("https://api.github.com/users/%s", username),
			ProfileURL: fmt.Sprintf("https://github.com/%s", username),
			Method:     "GET",
		},
		{
			Name:       "GitLab",
			Icon:       "🦊",
			CheckURL:   fmt.Sprintf("https://gitlab.com/%s", username),
			ProfileURL: fmt.Sprintf("https://gitlab.com/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Reddit",
			Icon:       "👾",
			CheckURL:   fmt.Sprintf("https://www.reddit.com/user/%s/about.json", username),
			ProfileURL: fmt.Sprintf("https://www.reddit.com/user/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Telegram",
			Icon:       "✈️",
			CheckURL:   fmt.Sprintf("https://t.me/%s", username),
			ProfileURL: fmt.Sprintf("https://t.me/%s", username),
			Method:     "GET",
		},
		{
			Name:       "DockerHub",
			Icon:       "🐳",
			CheckURL:   fmt.Sprintf("https://hub.docker.com/v2/users/%s/", username),
			ProfileURL: fmt.Sprintf("https://hub.docker.com/u/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Dev.to",
			Icon:       "👩‍💻",
			CheckURL:   fmt.Sprintf("https://dev.to/%s", username),
			ProfileURL: fmt.Sprintf("https://dev.to/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "HackerNews",
			Icon:       "🟠",
			CheckURL:   fmt.Sprintf("https://news.ycombinator.com/user?id=%s", username),
			ProfileURL: fmt.Sprintf("https://news.ycombinator.com/user?id=%s", username),
			Method:     "GET",
		},
		{
			Name:       "Medium",
			Icon:       "✍️",
			CheckURL:   fmt.Sprintf("https://medium.com/@%s", username),
			ProfileURL: fmt.Sprintf("https://medium.com/@%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Keybase",
			Icon:       "🗝️",
			CheckURL:   fmt.Sprintf("https://keybase.io/%s", username),
			ProfileURL: fmt.Sprintf("https://keybase.io/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "Chess.com",
			Icon:       "♟️",
			CheckURL:   fmt.Sprintf("https://api.chess.com/pub/player/%s", username),
			ProfileURL: fmt.Sprintf("https://www.chess.com/member/%s", username),
			Method:     "GET",
		},
		{
			Name:       "Codeberg",
			Icon:       "🏔️",
			CheckURL:   fmt.Sprintf("https://codeberg.org/%s", username),
			ProfileURL: fmt.Sprintf("https://codeberg.org/%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "NPM",
			Icon:       "📦",
			CheckURL:   fmt.Sprintf("https://www.npmjs.com/~%s", username),
			ProfileURL: fmt.Sprintf("https://www.npmjs.com/~%s", username),
			Method:     "HEAD",
		},
		{
			Name:       "PyPI",
			Icon:       "🐍",
			CheckURL:   fmt.Sprintf("https://pypi.org/user/%s/", username),
			ProfileURL: fmt.Sprintf("https://pypi.org/user/%s/", username),
			Method:     "HEAD",
		},
		{
			Name:       "Steam",
			Icon:       "🎮",
			CheckURL:   fmt.Sprintf("https://steamcommunity.com/id/%s", username),
			ProfileURL: fmt.Sprintf("https://steamcommunity.com/id/%s", username),
			Method:     "GET",
		},
	}

	client := &http.Client{
		Timeout: 1800 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	var wg sync.WaitGroup
	resultsChan := make(chan scanResult, len(platforms))

	for _, p := range platforms {
		wg.Add(1)
		go func(plat socialPlatform) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, plat.Method, plat.CheckURL, nil)
			if err != nil {
				resultsChan <- scanResult{Platform: plat, Found: false, URL: plat.ProfileURL}
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "*/*")

			resp, err := client.Do(req)
			if err != nil {
				resultsChan <- scanResult{Platform: plat, Found: false, URL: plat.ProfileURL}
				return
			}
			defer resp.Body.Close()

			found := (resp.StatusCode == 200)
			resultsChan <- scanResult{
				Platform: plat,
				Found:    found,
				URL:      plat.ProfileURL,
			}
		}(p)
	}

	wg.Wait()
	close(resultsChan)

	var foundList []scanResult
	var notFoundList []scanResult

	for res := range resultsChan {
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
		</a>`, item.URL, item.Platform.Icon, item.Platform.Name))
	}

	var missingHTML strings.Builder
	for _, item := range notFoundList {
		missingHTML.WriteString(fmt.Sprintf(`
		<span style="display:inline-flex; align-items:center; gap:0.3rem; padding:0.35rem 0.6rem; background:var(--bg-glass-card); border:1px solid var(--border-glass); border-radius:var(--radius-sm); color:var(--text-muted); font-size:0.8rem;">
			<span>%s %s</span>
		</span>`, item.Platform.Icon, item.Platform.Name))
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
		<div style="display:flex; flex-wrap:wrap; gap:0.4rem;">
			%s
		</div>
	</div>`, len(foundList), foundHTML.String(), len(notFoundList), missingHTML.String())

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🕵️ OSINT Social Username Recon: @%s", username),
		Value:       fmt.Sprintf("%d active accounts located across %d platforms", len(foundList), len(platforms)),
		Description: desc,
		URL:         fmt.Sprintf("https://github.com/%s", username),
	}
}
