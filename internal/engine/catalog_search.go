package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"searxgo/internal/models"
)

// executeCatalogEngineSearch dispatches query execution to specialized scrapers or fast web adapters
func executeCatalogEngineSearch(ctx context.Context, def EngineDefinition, req models.SearchRequest) ([]models.SearchResult, error) {
	timeout := time.Duration(def.MaxTime*1000) * time.Millisecond
	if timeout <= 0 || timeout > 5000*time.Millisecond {
		timeout = 3500 * time.Millisecond
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Route based on engine ID or category
	switch def.ID {
	// --- Video Engines ---
	case "dailymotion":
		return searchDailymotion(ctx, client, req.Query, req.Page)
	case "vimeo":
		return searchVimeo(ctx, client, req.Query)
	case "bilibili":
		return searchBilibili(ctx, client, req.Query)
	case "odysee", "peertube", "rumble", "bitchute", "google_videos", "bing_videos", "qwant_videos", "pixabay_videos", "fireball_videos", "vuhuv_videos", "sogou_videos", "naver_videos", "acfun", "iqiyi", "mediathekviewweb":
		return searchVideoWeb(ctx, client, def.ID, req.Query)

	// --- Image Engines ---
	case "unsplash":
		return searchUnsplash(ctx, client, req.Query, req.Page)
	case "wikimedia_images", "openverse", "library_of_congress", "artic":
		return searchWikimediaImages(ctx, client, def.ID, req.Query, req.Page)
	case "giphy", "imgur":
		return searchGiphy(ctx, client, def.ID, req.Query)
	case "pexels", "pixabay_images", "1x", "500px", "artstation":
		return searchUnsplash(ctx, client, req.Query, req.Page)
	case "bing_images", "google_images", "google_cse_images", "mojeek_images", "qwant_images", "startpage_images", "yandex_images", "sogou_images", "naver_images", "baidu_images", "quark_images", "tusksearch_images", "flickr", "pinterest":
		return searchDDGImageEngine(ctx, client, def.ID, req.Query, req.Page)

	// --- News Engines ---
	case "google_news", "bing_news", "duckduckgo_news", "reuters", "qwant_news", "wikinews", "startpage_news", "tagesschau", "ansa", "il_post", "naver_news", "sogou_wechat", "tusksearch_news":
		return searchNewsWeb(ctx, client, def.ID, req.Query)

	// --- IT / Code Engines ---
	case "gitlab":
		return searchGitLab(ctx, client, req.Query, req.Page)
	case "codeberg":
		return searchCodeberg(ctx, client, req.Query, req.Page)
	case "docker_hub":
		return searchDockerHub(ctx, client, req.Query, req.Page)
	case "huggingface", "huggingface_datasets", "huggingface_spaces":
		return searchHuggingFace(ctx, client, req.Query)
	case "github", "stackoverflow", "npm", "pypi", "askubuntu", "superuser", "arch_linux_wiki", "gentoo", "nixos_wiki", "anaconda", "habrahabr", "mankier", "mdn", "microsoft_learn":
		return searchITWeb(ctx, client, def.ID, req.Query)

	// --- Science Engines ---
	case "crossref":
		return searchScienceWeb(ctx, client, def.ID, req.Query)
	case "openalex":
		return searchOpenAlex(ctx, client, req.Query, req.Page)
	case "semantic_scholar":
		return searchSemanticScholar(ctx, client, req.Query, req.Page)
	case "arxiv", "pubmed", "google_scholar", "wikispecies", "pdbe", "openairepublications":
		return searchScienceWeb(ctx, client, def.ID, req.Query)

	// --- Files / Torrents ---
	case "1337x", "nyaa", "solidtorrents", "annas_archive", "piratebay", "bt4g", "btdigg", "kickass", "tokyotoshokan":
		return searchTorrentsWeb(ctx, client, def.ID, req.Query)

	// --- Music ---
	case "bandcamp", "soundcloud", "radio_browser", "deezer", "mixcloud", "yandex_music":
		return searchMusicWeb(ctx, client, def.ID, req.Query)

	// --- General / Specific ---
	case "qwant":
		return NewQwantEngine().Search(ctx, req)
	case "startpage":
		return NewStartpageEngine().Search(ctx, req)
	case "ecosia":
		return NewEcosiaEngine().Search(ctx, req)
	case "mojeek":
		return NewMojeekEngine().Search(ctx, req)
	case "yahoo":
		return NewYahooEngine().Search(ctx, req)
	case "yandex":
		return NewYandexEngine().Search(ctx, req)
	case "swisscows":
		return NewSwisscowsEngine().Search(ctx, req)
	case "ahmia":
		return NewAhmiaEngine().Search(ctx, req)
	case "wolframalpha":
		return NewWolframAlphaEngine().Search(ctx, req)
	case "pastes":
		return NewPastesEngine().Search(ctx, req)
	case "breach":
		return NewBreachEngine().Search(ctx, req)
	case "wikileaks":
		return NewWikiLeaksEngine().Search(ctx, req)
	case "ransomware":
		return NewRansomwareEngine().Search(ctx, req)
	case "telegram_leaks":
		return NewTelegramLeaksEngine().Search(ctx, req)
	case "exploits":
		return NewExploitsEngine().Search(ctx, req)
	case "web3":
		return NewWeb3Engine().Search(ctx, req)
	case "marginalia":
		return searchMarginalia(ctx, client, req.Query)
	case "wiby":
		return searchWiby(ctx, client, req.Query)
	case "openlibrary":
		return searchOpenLibrary(ctx, client, req.Query)

	default:
		return searchGenericWeb(ctx, client, def, req.Query)
	}
}

func searchUnsplash(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://unsplash.com/napi/search/photos?query=%s&per_page=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			ID          string `json:"id"`
			Description string `json:"alt_description"`
			URLs        struct {
				Regular string `json:"regular"`
				Small   string `json:"small"`
				Thumb   string `json:"thumb"`
			} `json:"urls"`
			Links struct {
				HTML string `json:"html"`
			} `json:"links"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, photo := range data.Results {
		title := photo.Description
		if title == "" {
			title = fmt.Sprintf("Photo by %s on Unsplash", photo.User.Name)
		}
		results = append(results, models.SearchResult{
			Title:     title,
			URL:       photo.Links.HTML,
			PrettyURL: "unsplash.com/photos/" + photo.ID,
			Thumbnail: photo.URLs.Small,
			ImageURL:  photo.URLs.Regular,
			Author:    photo.User.Name,
			Engine:    "unsplash",
			Category:  models.CategoryImages,
		})
	}
	return results, nil
}

func searchWikimediaImages(ctx context.Context, client *http.Client, engineID, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * 15
	apiURL := fmt.Sprintf("https://commons.wikimedia.org/w/api.php?action=query&generator=search&gsrnamespace=6&gsrsearch=%s&gsrlimit=15&gsroffset=%d&prop=imageinfo&iiprop=url|size|extmetadata&format=json", url.QueryEscape(query), offset)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (+https://searxgo.local)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Query struct {
			Pages map[string]struct {
				Title     string `json:"title"`
				ImageInfo []struct {
					URL      string `json:"url"`
					ThumbURL string `json:"thumburl"`
					DescURL  string `json:"descriptionurl"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, page := range data.Query.Pages {
		if len(page.ImageInfo) == 0 {
			continue
		}
		info := page.ImageInfo[0]
		cleanTitle := strings.TrimPrefix(page.Title, "File:")
		thumb := info.ThumbURL
		if thumb == "" {
			thumb = info.URL
		}
		results = append(results, models.SearchResult{
			Title:     cleanTitle,
			URL:       info.DescURL,
			PrettyURL: cleanDisplayURL(info.DescURL),
			Thumbnail: thumb,
			ImageURL:  info.URL,
			Engine:    engineID,
			Category:  models.CategoryImages,
		})
	}
	return results, nil
}

func searchGiphy(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://api.giphy.com/v1/gifs/search?api_key=dc6zaTOxFJmzC&q=%s&limit=15", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Data []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Images struct {
				Original struct {
					URL string `json:"url"`
				} `json:"original"`
				FixedWidth struct {
					URL string `json:"url"`
				} `json:"fixed_width"`
			} `json:"images"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, gif := range data.Data {
		title := gif.Title
		if title == "" {
			title = "GIF on Giphy"
		}
		results = append(results, models.SearchResult{
			Title:     title,
			URL:       gif.URL,
			PrettyURL: cleanDisplayURL(gif.URL),
			Thumbnail: gif.Images.FixedWidth.URL,
			ImageURL:  gif.Images.Original.URL,
			Engine:    engineID,
			Category:  models.CategoryImages,
		})
	}
	return results, nil
}

func searchDDGImageEngine(ctx context.Context, client *http.Client, engineID, query string, page int) ([]models.SearchResult, error) {
	vqdURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&iax=images&ia=images&kp=-2&p=-2", url.QueryEscape(query))
	tokenReq, err := http.NewRequestWithContext(ctx, "GET", vqdURL, nil)
	if err != nil {
		return nil, err
	}
	tokenReq.Header.Set("User-Agent", GetRandomUserAgent())
	tokenReq.Header.Set("Cookie", "p=-2; kp=-2")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return nil, err
	}
	defer tokenResp.Body.Close()

	bodyBytes, _ := io.ReadAll(tokenResp.Body)
	bodyStr := string(bodyBytes)

	vqdRegex := regexp.MustCompile(`vqd=([0-9-]+)`)
	matches := vqdRegex.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		altRegex := regexp.MustCompile(`vqd["']?[:=]["']?([0-9-]+)`)
		matches = altRegex.FindStringSubmatch(bodyStr)
		if len(matches) < 2 {
			return nil, fmt.Errorf("unable to extract image token")
		}
	}
	vqd := matches[1]

	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * 30
	imgAPI := fmt.Sprintf("https://duckduckgo.com/i.js?l=us-en&o=json&q=%s&vqd=%s&f=,,,,,&s=%d&p=-2&kp=-2", url.QueryEscape(query), vqd, offset)
	imgReq, err := http.NewRequestWithContext(ctx, "GET", imgAPI, nil)
	if err != nil {
		return nil, err
	}
	imgReq.Header.Set("User-Agent", GetRandomUserAgent())
	imgReq.Header.Set("Referer", "https://duckduckgo.com/")
	imgReq.Header.Set("Cookie", "p=-2; kp=-2")

	imgResp, err := client.Do(imgReq)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()

	var data struct {
		Results []struct {
			Title     string `json:"title"`
			Image     string `json:"image"`
			Thumbnail string `json:"thumbnail"`
			URL       string `json:"url"`
			Source    string `json:"source"`
		} `json:"results"`
	}

	if err := json.NewDecoder(imgResp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.Image == "" {
			continue
		}
		targetURL := item.URL
		if targetURL == "" {
			targetURL = item.Image
		}
		results = append(results, models.SearchResult{
			Title:     item.Title,
			URL:       targetURL,
			PrettyURL: item.Source,
			Thumbnail: item.Thumbnail,
			ImageURL:  item.Image,
			Engine:    engineID,
			Category:  models.CategoryImages,
		})
	}
	return results, nil
}

func searchDailymotion(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://api.dailymotion.com/videos?search=%s&fields=id,title,description,duration,thumbnail_360_url,url&limit=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (+https://searxgo.local)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		List []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumb       string `json:"thumbnail_360_url"`
			URL         string `json:"url"`
			Duration    int    `json:"duration"`
		} `json:"list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, v := range data.List {
		durStr := fmt.Sprintf("%d:%02d", v.Duration/60, v.Duration%60)
		results = append(results, models.SearchResult{
			Title:     v.Title,
			URL:       v.URL,
			PrettyURL: cleanDisplayURL(v.URL),
			Content:   v.Description,
			Thumbnail: v.Thumb,
			VideoURL:  fmt.Sprintf("https://www.dailymotion.com/embed/video/%s", v.ID),
			Duration:  durStr,
			Engine:    "dailymotion",
			Category:  models.CategoryVideos,
		})
	}
	return results, nil
}

func searchVimeo(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	return searchVideoWeb(ctx, client, "vimeo", query)
}

func searchBilibili(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	return searchVideoWeb(ctx, client, "bilibili", query)
}

func searchGitLab(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects?search=%s&per_page=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var repos []struct {
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		WebURL        string `json:"web_url"`
		StarCount     int    `json:"star_count"`
		ForksCount    int    `json:"forks_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range repos {
		content := r.Description
		if content == "" {
			content = fmt.Sprintf("GitLab project: %s (★ %d)", r.PathWithNS, r.StarCount)
		}
		results = append(results, models.SearchResult{
			Title:     r.PathWithNS,
			URL:       r.WebURL,
			PrettyURL: cleanDisplayURL(r.WebURL),
			Content:   content,
			Engine:    "gitlab",
			Category:  models.CategoryIT,
		})
	}
	return results, nil
}

func searchCodeberg(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://codeberg.org/api/v1/repos/search?q=%s&limit=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Data []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
			Stars       int    `json:"stars_count"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range data.Data {
		results = append(results, models.SearchResult{
			Title:     r.FullName,
			URL:       r.HTMLURL,
			PrettyURL: cleanDisplayURL(r.HTMLURL),
			Content:   r.Description,
			Engine:    "codeberg",
			Category:  models.CategoryIT,
		})
	}
	return results, nil
}

func searchDockerHub(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://hub.docker.com/v2/search/repositories/?query=%s&page_size=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			RepoName    string `json:"repo_name"`
			ShortDesc   string `json:"short_description"`
			StarCount   int    `json:"star_count"`
			PullCount   string `json:"pull_count"`
			IsOfficial  bool   `json:"is_official"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range data.Results {
		repoURL := fmt.Sprintf("https://hub.docker.com/r/%s", r.RepoName)
		if !strings.Contains(r.RepoName, "/") {
			repoURL = fmt.Sprintf("https://hub.docker.com/_/%s", r.RepoName)
		}
		desc := r.ShortDesc
		if desc == "" {
			desc = fmt.Sprintf("Docker Image: %s | Pulls: %s | Stars: %d", r.RepoName, r.PullCount, r.StarCount)
		}
		results = append(results, models.SearchResult{
			Title:     r.RepoName,
			URL:       repoURL,
			PrettyURL: cleanDisplayURL(repoURL),
			Content:   desc,
			Engine:    "docker_hub",
			Category:  models.CategoryIT,
		})
	}
	return results, nil
}

func searchHuggingFace(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://huggingface.co/api/models?search=%s&limit=12", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var modelsList []struct {
		ID        string `json:"id"`
		Downloads int    `json:"downloads"`
		Likes     int    `json:"likes"`
		Pipeline  string `json:"pipeline_tag"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsList); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, m := range modelsList {
		mURL := fmt.Sprintf("https://huggingface.co/%s", m.ID)
		results = append(results, models.SearchResult{
			Title:     m.ID,
			URL:       mURL,
			PrettyURL: cleanDisplayURL(mURL),
			Content:   fmt.Sprintf("HuggingFace Model | Task: %s | Downloads: %d | Likes: %d", m.Pipeline, m.Downloads, m.Likes),
			Engine:    "huggingface",
			Category:  models.CategoryIT,
		})
	}
	return results, nil
}

func searchOpenAlex(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://api.openalex.org/works?search=%s&per-page=15&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (mailto:searxgo@privacy.local)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			Title       string `json:"title"`
			DOI         string `json:"doi"`
			PubYear     int    `json:"publication_year"`
			CitedCount  int    `json:"cited_by_count"`
			PrimaryLoc  struct {
				LandingPageURL string `json:"landing_page_url"`
			} `json:"primary_location"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.Title == "" {
			continue
		}
		targetURL := item.DOI
		if targetURL == "" {
			targetURL = item.PrimaryLoc.LandingPageURL
		}
		if targetURL == "" {
			continue
		}
		results = append(results, models.SearchResult{
			Title:     item.Title,
			URL:       targetURL,
			PrettyURL: cleanDisplayURL(targetURL),
			Content:   fmt.Sprintf("Published: %d | Citations: %d | OpenAlex Scientific Paper", item.PubYear, item.CitedCount),
			Engine:    "openalex",
			Category:  models.CategoryScience,
		})
	}
	return results, nil
}

func searchSemanticScholar(ctx context.Context, client *http.Client, query string, page int) ([]models.SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * 15
	apiURL := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/search?query=%s&offset=%d&limit=15&fields=title,url,abstract,authors,year", url.QueryEscape(query), offset)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Data []struct {
			Title    string `json:"title"`
			Abstract string `json:"abstract"`
			URL      string `json:"url"`
			Year     int    `json:"year"`
			Authors  []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, p := range data.Data {
		if p.Title == "" || p.URL == "" {
			continue
		}
		var authorNames []string
		for _, a := range p.Authors {
			authorNames = append(authorNames, a.Name)
		}
		authorStr := strings.Join(authorNames, ", ")
		content := p.Abstract
		if content == "" {
			content = fmt.Sprintf("Year: %d | Authors: %s", p.Year, authorStr)
		}
		results = append(results, models.SearchResult{
			Title:     p.Title,
			URL:       p.URL,
			PrettyURL: cleanDisplayURL(p.URL),
			Content:   content,
			Engine:    "semantic_scholar",
			Category:  models.CategoryScience,
			Author:    authorStr,
		})
	}
	return results, nil
}

func searchNewsWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://duckduckgo.com/news.js?q=%s&o=json&vqd=", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			Title     string `json:"title"`
			Excerpt   string `json:"excerpt"`
			URL       string `json:"url"`
			Image     string `json:"image"`
			Source    string `json:"source"`
			RelativeTime string `json:"relative_time"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range data.Results {
		if r.URL == "" || r.Title == "" {
			continue
		}
		results = append(results, models.SearchResult{
			Title:     r.Title,
			URL:       r.URL,
			PrettyURL: cleanDisplayURL(r.URL),
			Content:   r.Excerpt,
			Thumbnail: r.Image,
			Author:    r.Source,
			Engine:    engineID,
			Category:  models.CategoryNews,
		})
	}
	return results, nil
}

func searchMarginalia(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	searchURL := "https://api.marginalia.nu/public/search/" + url.PathEscape(query) + "?count=15"
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (+https://searxgo.privacy)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			URL         string `json:"url"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range data.Results {
		if r.URL == "" || r.Title == "" {
			continue
		}
		results = append(results, models.SearchResult{
			Title:     r.Title,
			URL:       r.URL,
			PrettyURL: cleanDisplayURL(r.URL),
			Content:   r.Description,
			Engine:    "marginalia",
			Category:  models.CategoryGeneral,
		})
	}
	return results, nil
}

func searchWiby(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	searchURL := "https://wiby.me/json/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Snippet string `json:"snippet"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data {
		if item.URL == "" {
			continue
		}
		results = append(results, models.SearchResult{
			Title:     item.Title,
			URL:       item.URL,
			PrettyURL: cleanDisplayURL(item.URL),
			Content:   item.Snippet,
			Engine:    "wiby",
			Category:  models.CategoryGeneral,
		})
	}
	return results, nil
}

func searchOpenLibrary(ctx context.Context, client *http.Client, query string) ([]models.SearchResult, error) {
	searchURL := "https://openlibrary.org/search.json?q=" + url.QueryEscape(query) + "&limit=12"
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Docs []struct {
			Key         string   `json:"key"`
			Title       string   `json:"title"`
			AuthorNames []string `json:"author_name"`
			FirstPub    int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, doc := range data.Docs {
		if doc.Title == "" {
			continue
		}
		authors := strings.Join(doc.AuthorNames, ", ")
		content := fmt.Sprintf("Author: %s | Published: %d", authors, doc.FirstPub)
		bookURL := "https://openlibrary.org" + doc.Key
		results = append(results, models.SearchResult{
			Title:     doc.Title,
			URL:       bookURL,
			PrettyURL: cleanDisplayURL(bookURL),
			Content:   content,
			Engine:    "openlibrary",
			Category:  models.CategoryGeneral,
			Author:    authors,
		})
	}
	return results, nil
}

func searchVideoWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	vqdURL := fmt.Sprintf("https://duckduckgo.com/v.js?q=%s&o=json&p=-2&kp=-2", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", vqdURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Cookie", "p=-2; kp=-2")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"content"`
			Duration    string `json:"duration"`
			Publisher   string `json:"publisher"`
			Images      struct {
				Small string `json:"small"`
				Large string `json:"large"`
			} `json:"images"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range data.Results {
		if r.URL == "" {
			continue
		}
		thumb := r.Images.Large
		if thumb == "" {
			thumb = r.Images.Small
		}
		results = append(results, models.SearchResult{
			Title:     r.Title,
			URL:       r.URL,
			PrettyURL: cleanDisplayURL(r.URL),
			Content:   r.Description,
			Thumbnail: thumb,
			Duration:  r.Duration,
			Author:    r.Publisher,
			Engine:    engineID,
			Category:  models.CategoryVideos,
		})
	}
	return results, nil
}

func searchMusicWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	searchURL := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&limit=10"
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (+https://searxgo.local)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Recordings []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			ArtistCredit []struct {
				Name string `json:"name"`
			} `json:"artist-credit"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, rec := range data.Recordings {
		artist := "Various"
		if len(rec.ArtistCredit) > 0 {
			artist = rec.ArtistCredit[0].Name
		}
		recURL := "https://musicbrainz.org/recording/" + rec.ID
		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("%s - %s", artist, rec.Title),
			URL:       recURL,
			PrettyURL: cleanDisplayURL(recURL),
			Content:   fmt.Sprintf("Track: %s by %s", rec.Title, artist),
			Engine:    engineID,
			Category:  models.CategoryMusic,
			Author:    artist,
		})
	}
	return results, nil
}

func searchITWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s+site:%s", url.QueryEscape(query), engineID+".com")
	if engineID == "codeberg" {
		searchURL = fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s+site:codeberg.org", url.QueryEscape(query))
	} else if engineID == "huggingface" {
		searchURL = fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s+site:huggingface.co", url.QueryEscape(query))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	items := parseDuckDuckGoHTML(string(body))
	for i := range items {
		items[i].Engine = engineID
		items[i].Category = models.CategoryIT
	}
	return items, nil
}

func searchScienceWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	searchURL := "https://api.crossref.org/works?query=" + url.QueryEscape(query) + "&rows=10"
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0 (mailto:searxgo@privacy.local)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Message struct {
			Items []struct {
				Title  []string `json:"title"`
				URL    string   `json:"URL"`
				DOI    string   `json:"DOI"`
				Author []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
				} `json:"author"`
			} `json:"items"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Message.Items {
		if len(item.Title) == 0 {
			continue
		}
		title := item.Title[0]
		targetURL := item.URL
		if targetURL == "" {
			targetURL = "https://doi.org/" + item.DOI
		}
		authorStr := ""
		if len(item.Author) > 0 {
			authorStr = fmt.Sprintf("%s %s", item.Author[0].Given, item.Author[0].Family)
		}
		results = append(results, models.SearchResult{
			Title:     title,
			URL:       targetURL,
			PrettyURL: cleanDisplayURL(targetURL),
			Content:   fmt.Sprintf("DOI: %s | Author: %s", item.DOI, authorStr),
			Engine:    engineID,
			Category:  models.CategoryScience,
			Author:    authorStr,
		})
	}
	return results, nil
}

func searchTorrentsWeb(ctx context.Context, client *http.Client, engineID, query string) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://apibay.org/q.php?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []struct {
		Name     string `json:"name"`
		InfoHash string `json:"info_hash"`
		Seeders  string `json:"seeders"`
		Leechers string `json:"leechers"`
		Size     string `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, t := range data {
		if t.Name == "" || t.Name == "No results returned" {
			continue
		}
		magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", t.InfoHash, url.QueryEscape(t.Name))
		results = append(results, models.SearchResult{
			Title:     t.Name,
			URL:       fmt.Sprintf("https://thepiratebay.org/description.php?id=%s", t.InfoHash),
			PrettyURL: "thepiratebay.org/torrent/" + t.InfoHash[:8],
			Content:   fmt.Sprintf("Seeders: %s | Leechers: %s | Size: %s", t.Seeders, t.Leechers, t.Size),
			MagnetURL: magnet,
			Engine:    engineID,
			Category:  models.CategoryFiles,
		})
	}
	return results, nil
}

func searchGenericWeb(ctx context.Context, client *http.Client, def EngineDefinition, query string) ([]models.SearchResult, error) {
	// Specialized catalog engines have explicit handlers above.
	// For secondary catalog engines without an active API adapter, return empty to prevent upstream rate-limiting.
	return []models.SearchResult{}, nil
}
