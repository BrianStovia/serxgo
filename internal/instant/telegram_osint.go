package instant

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

var (
	tgChannelRegex = regexp.MustCompile(`^(?:(?:https?:\/\/)?t(?:elegram)?\.me\/(?:s\/)?|@|tg:\s*|tg\s+|telegram:\s*|telegram\s+|tlgrm:\s*)([a-zA-Z0-9_]{4,32})`)
)

type TelegramChannelProfile struct {
	Handle       string
	Title        string
	AvatarURL    string
	Subscribers  string
	Description  string
	IsVerified   bool
	RecentPosts  []TelegramPostSnippet
	ChannelURL   string
	WebPreviewURL string
}

type TelegramPostSnippet struct {
	Text      string
	PostURL   string
	Views     string
	Datetime  string
}

// CheckTelegramOSINTQuery checks if query targets a Telegram channel, username, or OSINT lookup
func CheckTelegramOSINTQuery(ctx context.Context, query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	var channelHandle string

	// 1. Explicit prefixes: "tg: @channel", "tg channel", "telegram: handle", "t.me/handle", "t.me/s/handle"
	explicitPrefixes := []string{"tg:", "tg ", "telegram:", "telegram ", "tlgrm:", "tme:", "tme "}
	for _, p := range explicitPrefixes {
		if strings.HasPrefix(qLower, p) {
			raw := strings.TrimSpace(q[len(p):])
			raw = strings.TrimPrefix(raw, "@")
			raw = strings.TrimPrefix(raw, "https://t.me/s/")
			raw = strings.TrimPrefix(raw, "https://t.me/")
			raw = strings.TrimPrefix(raw, "http://t.me/s/")
			raw = strings.TrimPrefix(raw, "http://t.me/")
			raw = strings.TrimPrefix(raw, "t.me/s/")
			raw = strings.TrimPrefix(raw, "t.me/")
			if idx := strings.Index(raw, "/"); idx != -1 {
				raw = raw[:idx]
			}
			if idx := strings.Index(raw, " "); idx != -1 {
				raw = raw[:idx]
			}
			channelHandle = cleanTelegramHandle(raw)
			break
		}
	}

	// 2. Direct t.me URL format
	if channelHandle == "" && (strings.HasPrefix(qLower, "t.me/") || strings.HasPrefix(qLower, "https://t.me/") || strings.HasPrefix(qLower, "http://t.me/")) {
		matches := tgChannelRegex.FindStringSubmatch(q)
		if len(matches) > 1 {
			channelHandle = cleanTelegramHandle(matches[1])
		}
	}

	// 3. Standalone "@handle" (must not be an email address)
	if channelHandle == "" && strings.HasPrefix(q, "@") && !strings.Contains(q, " ") && !strings.Contains(q, ".") && len(q) >= 4 {
		channelHandle = cleanTelegramHandle(strings.TrimPrefix(q, "@"))
	}

	if channelHandle == "" {
		return nil
	}

	// Fetch and parse live channel profile
	profile, err := FetchTelegramChannelProfile(ctx, channelHandle)
	if err != nil || profile == nil || profile.Title == "" {
		// Fallback: Build minimal OSINT card if live scraping is rate-limited or blocked
		return buildMinimalTelegramCard(channelHandle)
	}

	return formatTelegramOSINTCard(profile)
}

func cleanTelegramHandle(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "@")
	validRegex := regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	if validRegex.MatchString(raw) {
		return raw
	}
	return ""
}

// FetchTelegramChannelProfile scrapes Telegram's public web preview at https://t.me/s/<channel>
func FetchTelegramChannelProfile(ctx context.Context, handle string) (*TelegramChannelProfile, error) {
	reqURL := fmt.Sprintf("https://t.me/s/%s", handle)
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	profile := &TelegramChannelProfile{
		Handle:        handle,
		ChannelURL:    fmt.Sprintf("https://t.me/%s", handle),
		WebPreviewURL: fmt.Sprintf("https://t.me/s/%s", handle),
	}

	// Extract Title
	titleNode := findFirstNodeWithClass(doc, "tgme_page_title")
	if titleNode != nil {
		profile.Title = extractNodeText(titleNode)
	}
	if profile.Title == "" {
		// Try header title tag
		if hNode := findFirstNodeTag(doc, "title"); hNode != nil {
			profile.Title = extractNodeText(hNode)
			profile.Title = strings.TrimSuffix(profile.Title, " – Telegram")
		}
	}

	// Extract Verified Status
	if verifiedNode := findFirstNodeWithClass(doc, "tgme_icon_verified"); verifiedNode != nil {
		profile.IsVerified = true
	}

	// Extract Subscribers / Members
	extraNode := findFirstNodeWithClass(doc, "tgme_page_extra")
	if extraNode != nil {
		profile.Subscribers = extractNodeText(extraNode)
	}

	// Extract Description / Bio
	descNode := findFirstNodeWithClass(doc, "tgme_page_description")
	if descNode != nil {
		profile.Description = extractNodeText(descNode)
	}

	// Extract Avatar
	photoNode := findFirstNodeWithClass(doc, "tgme_page_photo_image")
	if photoNode != nil {
		profile.AvatarURL = getNodeAttribute(photoNode, "src")
	}

	// Extract Recent Message Previews (up to 3)
	msgNodes := findNodesWithClass(doc, "tgme_widget_message_wrap")
	for i := len(msgNodes) - 1; i >= 0 && len(profile.RecentPosts) < 3; i-- {
		mNode := msgNodes[i]
		textNode := findFirstNodeWithClass(mNode, "tgme_widget_message_text")
		if textNode == nil {
			continue
		}
		text := extractNodeText(textNode)
		if len(text) > 200 {
			text = text[:197] + "..."
		}

		views := ""
		if vNode := findFirstNodeWithClass(mNode, "tgme_widget_message_views"); vNode != nil {
			views = extractNodeText(vNode)
		}

		dt := ""
		if dtNode := findFirstNodeTag(mNode, "time"); dtNode != nil {
			dt = getNodeAttribute(dtNode, "datetime")
			if dt == "" {
				dt = extractNodeText(dtNode)
			}
		}

		profile.RecentPosts = append(profile.RecentPosts, TelegramPostSnippet{
			Text:     text,
			Views:    views,
			Datetime: dt,
		})
	}

	return profile, nil
}

func buildMinimalTelegramCard(handle string) *models.InstantAnswer {
	html := fmt.Sprintf(`
<div style="font-family:var(--font-main); font-size:0.875rem;">
  <div style="display:flex; align-items:center; gap:0.75rem; margin-bottom:0.75rem;">
    <div style="width:42px; height:42px; border-radius:50%%; background:linear-gradient(135deg, #0088cc, #00b4d8); display:flex; align-items:center; justify-content:center; color:white; font-weight:700; font-size:1.1rem;">
      TG
    </div>
    <div>
      <div style="font-size:1.1rem; font-weight:700; color:var(--text-primary);">@%s</div>
      <div style="font-size:0.8rem; color:var(--text-secondary);">Telegram Public Channel / User Profile</div>
    </div>
  </div>

  <div style="display:flex; flex-wrap:wrap; gap:0.5rem; margin-top:0.75rem;">
    <a href="https://t.me/s/%s" target="_blank" rel="noopener noreferrer" style="background:#0088cc; color:white; padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-weight:600; font-size:0.8rem;">
      🌐 Open Web Preview (t.me/s)
    </a>
    <a href="tg://resolve?domain=%s" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-weight:600; font-size:0.8rem;">
      📲 Launch in App
    </a>
    <a href="https://tgstat.com/channel/@%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      📊 TGStat Analytics
    </a>
    <a href="https://telemetr.io/en/channels?search=%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      📈 Telemetr Intel
    </a>
    <a href="https://intelx.io/?s=t.me%%2F%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--accent-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      🔍 IntelX Archive
    </a>
  </div>
</div>`, handle, handle, handle, handle, handle, handle)

	return &models.InstantAnswer{
		Type:        "telegram_osint",
		Title:       fmt.Sprintf("📱 Telegram OSINT Profile: @%s", handle),
		Value:       fmt.Sprintf("Telegram Target: @%s", handle),
		Description: html,
		URL:         fmt.Sprintf("https://t.me/s/%s", handle),
	}
}

func formatTelegramOSINTCard(p *TelegramChannelProfile) *models.InstantAnswer {
	verifiedBadge := ""
	if p.IsVerified {
		verifiedBadge = `<span style="display:inline-flex; align-items:center; justify-content:center; width:18px; height:18px; background:#0088cc; color:white; border-radius:50%; font-size:0.75rem; margin-left:0.3rem;" title="Verified Channel">✓</span>`
	}

	avatarHTML := `<div style="width:48px; height:48px; border-radius:50%; background:linear-gradient(135deg, #0088cc, #00b4d8); display:flex; align-items:center; justify-content:center; color:white; font-weight:700; font-size:1.2rem; flex-shrink:0;">TG</div>`
	if p.AvatarURL != "" {
		avatarHTML = fmt.Sprintf(`<img src="/proxy/image?url=%s" alt="@%s" style="width:48px; height:48px; border-radius:50%%; object-fit:cover; border:2px solid var(--accent-primary); flex-shrink:0;" onerror="this.style.display='none';" />`, url.QueryEscape(p.AvatarURL), p.Handle)
	}

	subscribersHTML := ""
	if p.Subscribers != "" {
		subscribersHTML = fmt.Sprintf(`<span style="background:rgba(0,136,204,0.15); color:#00a8ff; border:1px solid rgba(0,136,204,0.3); padding:2px 8px; border-radius:12px; font-size:0.75rem; font-weight:600;">👥 %s</span>`, p.Subscribers)
	}

	descHTML := ""
	if p.Description != "" {
		descHTML = fmt.Sprintf(`<div style="background:var(--bg-glass-card); border-left:3px solid var(--accent-primary); padding:0.6rem 0.85rem; border-radius:var(--radius-sm); margin:0.75rem 0; font-size:0.825rem; line-height:1.4; color:var(--text-secondary); word-break:break-word;">%s</div>`, escapeHTML(p.Description))
	}

	postsHTML := ""
	if len(p.RecentPosts) > 0 {
		var postItems []string
		for _, post := range p.RecentPosts {
			viewsBadge := ""
			if post.Views != "" {
				viewsBadge = fmt.Sprintf(`<span style="font-size:0.7rem; color:var(--text-muted); margin-left:auto;">👁️ %s</span>`, post.Views)
			}
			postItems = append(postItems, fmt.Sprintf(`
        <div style="padding:0.4rem 0.6rem; background:rgba(255,255,255,0.03); border:1px solid var(--border-glow); border-radius:var(--radius-sm); font-size:0.8rem; margin-bottom:0.35rem;">
          <div style="color:var(--text-primary); line-height:1.3; margin-bottom:0.25rem;">%s</div>
          <div style="display:flex; align-items:center; font-size:0.7rem; color:var(--text-muted);">
            <span>🕒 %s</span>
            %s
          </div>
        </div>`, escapeHTML(post.Text), post.Datetime, viewsBadge))
		}
		postsHTML = fmt.Sprintf(`
      <div style="margin-top:0.75rem;">
        <div style="font-size:0.8rem; font-weight:700; color:var(--text-primary); margin-bottom:0.4rem; text-transform:uppercase; letter-spacing:0.05em;">💬 Recent Public Feed Previews</div>
        %s
      </div>`, strings.Join(postItems, ""))
	}

	html := fmt.Sprintf(`
<div style="font-family:var(--font-main); font-size:0.875rem;">
  <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:0.5rem; margin-bottom:0.5rem;">
    <div style="display:flex; align-items:center; gap:0.75rem;">
      %s
      <div>
        <div style="display:flex; align-items:center; font-size:1.15rem; font-weight:700; color:var(--text-primary);">
          %s %s
        </div>
        <div style="display:flex; align-items:center; gap:0.5rem; margin-top:0.15rem;">
          <span style="color:var(--accent-primary); font-weight:600; font-size:0.85rem;">@%s</span>
          %s
        </div>
      </div>
    </div>
  </div>

  %s
  %s

  <!-- OSINT Action Deep Links -->
  <div style="display:flex; flex-wrap:wrap; gap:0.5rem; margin-top:0.9rem; padding-top:0.75rem; border-top:1px solid var(--border-glow);">
    <a href="https://t.me/s/%s" target="_blank" rel="noopener noreferrer" style="background:#0088cc; color:white; padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-weight:600; font-size:0.8rem;">
      🌐 Open Web Preview (t.me/s)
    </a>
    <a href="tg://resolve?domain=%s" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-weight:600; font-size:0.8rem;">
      📲 Launch in App
    </a>
    <a href="https://tgstat.com/channel/@%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      📊 TGStat Analytics
    </a>
    <a href="https://telemetr.io/en/channels?search=%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      📈 Telemetr Intel
    </a>
    <a href="https://intelx.io/?s=t.me%%2F%s" target="_blank" rel="noopener noreferrer" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--accent-primary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      🔍 IntelX Leak Search
    </a>
    <a href="/search?q=%s" style="background:var(--bg-glass-card); border:1px solid var(--border-glow); color:var(--text-secondary); padding:0.4rem 0.85rem; border-radius:var(--radius-sm); text-decoration:none; font-size:0.8rem;">
      🕵️ Dork Channel Content
    </a>
  </div>
</div>`, avatarHTML, escapeHTML(p.Title), verifiedBadge, p.Handle, subscribersHTML, descHTML, postsHTML, p.Handle, p.Handle, p.Handle, p.Handle, p.Handle, url.QueryEscape(fmt.Sprintf("site:t.me/s/%s", p.Handle)))

	return &models.InstantAnswer{
		Type:        "telegram_osint",
		Title:       fmt.Sprintf("📱 Telegram OSINT Intelligence: @%s", p.Handle),
		Value:       fmt.Sprintf("%s (@%s)", p.Title, p.Handle),
		Description: html,
		URL:         p.WebPreviewURL,
		Thumbnail:   p.AvatarURL,
	}
}

// DOM Helper functions for Telegram web HTML parsing
func findFirstNodeWithClass(n *xhtml.Node, className string) *xhtml.Node {
	if n == nil {
		return nil
	}
	if n.Type == xhtml.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "class" && strings.Contains(a.Val, className) {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findFirstNodeWithClass(c, className); res != nil {
			return res
		}
	}
	return nil
}

func findNodesWithClass(n *xhtml.Node, className string) []*xhtml.Node {
	var nodes []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil {
			return
		}
		if node.Type == xhtml.ElementNode {
			for _, a := range node.Attr {
				if a.Key == "class" && strings.Contains(a.Val, className) {
					nodes = append(nodes, node)
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return nodes
}

func findFirstNodeTag(n *xhtml.Node, tagName string) *xhtml.Node {
	if n == nil {
		return nil
	}
	if n.Type == xhtml.ElementNode && n.Data == tagName {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findFirstNodeTag(c, tagName); res != nil {
			return res
		}
	}
	return nil
}

func getNodeAttribute(n *xhtml.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractNodeText(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
