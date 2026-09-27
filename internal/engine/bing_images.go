package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"searxgo/internal/models"
)

type BingImagesEngine struct {
	client *http.Client
}

func NewBingImagesEngine() *BingImagesEngine {
	return &BingImagesEngine{
		client: NewHTTPClient(4500 * time.Millisecond),
	}
}

func (e *BingImagesEngine) Name() string {
	return "bing_images"
}

func (e *BingImagesEngine) DisplayName() string {
	return "Bing Images"
}

func (e *BingImagesEngine) Categories() []models.Category {
	return []models.Category{models.CategoryImages}
}

func (e *BingImagesEngine) DefaultOn() bool {
	return true
}

func (e *BingImagesEngine) Weight() float64 {
	return 1.3
}

func (e *BingImagesEngine) About() string {
	return "Microsoft Bing official high-resolution image index."
}

var (
	rxBingImgM    = regexp.MustCompile(`class="iusc"[^>]*\sm="([^"]+)"`)
	rxBingImgSrc  = regexp.MustCompile(`data-src="([^"]+)"`)
	rxBingImgAlt  = regexp.MustCompile(`alt="([^"]*)"`)
	rxBingImgHref = regexp.MustCompile(`href="([^"]+)"`)
)

type bingMetadata struct {
	MURL string `json:"murl"` // original full image url
	TURL string `json:"turl"` // thumbnail url
	PURL string `json:"purl"` // publisher / page url
	Desc string `json:"desc"` // description / title
}

func (e *BingImagesEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	adlt := "off"
	if req.SafeSearch == models.SafeSearchStrict {
		adlt = "strict"
	} else if req.SafeSearch == models.SafeSearchModerate {
		adlt = "moderate"
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	first := (page-1)*35 + 1

	searchURL := fmt.Sprintf("https://www.bing.com/images/async?q=%s&first=%d&count=35&adlt=%s",
		url.QueryEscape(req.Query), first, adlt)

	isID := req.Language == "id" || req.Country == "ID" || req.Region == "id" || IsIndonesianText(req.Query)
	if isID {
		searchURL += "&setlang=id&cc=ID"
	} else if req.Language != "" {
		searchURL += fmt.Sprintf("&setlang=%s", url.QueryEscape(req.Language))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	acceptLang := "en-US,en;q=0.9"
	if isID {
		acceptLang = "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7"
	}
	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", acceptLang)
	httpReq.Header.Set("Cookie", fmt.Sprintf("SRCHHPGUSR=ADLT=%s", strings.ToUpper(adlt)))

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bing images returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	bodyStr := string(bodyBytes)

	// Bing images async response contains <div class="imgpt">...<a class="iusc" m="{...}">...<img data-src="..." alt="...">
	matches := rxBingImgM.FindAllStringSubmatch(bodyStr, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	var results []models.SearchResult
	seenURLs := make(map[string]bool)

	// Split chunks by <li or <div class="imgpt" to match metadata with thumbnail data-src
	chunks := strings.Split(bodyStr, "class=\"imgpt\"")

	for i, chunk := range chunks {
		if i == 0 {
			continue
		}

		mMatch := rxBingImgM.FindStringSubmatch(chunk)
		if len(mMatch) < 2 {
			continue
		}

		rawJSON := html.UnescapeString(mMatch[1])
		var meta bingMetadata
		if err := json.Unmarshal([]byte(rawJSON), &meta); err != nil {
			continue
		}

		imageURL := meta.MURL
		if imageURL == "" || seenURLs[imageURL] {
			continue
		}
		seenURLs[imageURL] = true

		thumbnail := meta.TURL
		if thumbnail == "" {
			srcMatch := rxBingImgSrc.FindStringSubmatch(chunk)
			if len(srcMatch) >= 2 {
				thumbnail = html.UnescapeString(srcMatch[1])
			}
		}
		if thumbnail == "" {
			thumbnail = imageURL
		}

		title := meta.Desc
		if title == "" {
			altMatch := rxBingImgAlt.FindStringSubmatch(chunk)
			if len(altMatch) >= 2 {
				title = html.UnescapeString(altMatch[1])
				// Clean formatting artifacts like  and 
				title = strings.ReplaceAll(title, "\uE000", "")
				title = strings.ReplaceAll(title, "\uE001", "")
			}
		}
		if title == "" {
			title = req.Query + " Image"
		}

		pageURL := meta.PURL
		if pageURL == "" {
			hrefMatch := rxBingImgHref.FindStringSubmatch(chunk)
			if len(hrefMatch) >= 2 {
				href := html.UnescapeString(hrefMatch[1])
				if strings.HasPrefix(href, "http") {
					pageURL = href
				} else {
					pageURL = "https://www.bing.com" + href
				}
			} else {
				pageURL = imageURL
			}
		}

		sourceHost := cleanDisplayURL(pageURL)

		results = append(results, models.SearchResult{
			Title:     strings.TrimSpace(title),
			URL:       pageURL,
			ImageURL:  imageURL,
			Thumbnail: thumbnail,
			Engine:    e.Name(),
			Category:  models.CategoryImages,
			PrettyURL: sourceHost,
		})
	}

	return results, nil
}
