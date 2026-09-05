package models

import "time"

// Category represents a search category matching all SearXNG categories
type Category string

const (
	CategoryGeneral Category = "general"
	CategoryImages  Category = "images"
	CategoryVideos  Category = "videos"
	CategoryNews    Category = "news"
	CategoryIT      Category = "it"
	CategoryScience Category = "science"
	CategorySocial  Category = "social"
	CategoryFiles   Category = "files"
	CategoryMusic   Category = "music"
	CategoryMaps    Category = "maps"
	CategoryOther   Category = "other"
)

// SafeSearchLevel represents safe search filtering mode
type SafeSearchLevel int

const (
	SafeSearchOff      SafeSearchLevel = 0
	SafeSearchModerate SafeSearchLevel = 1
	SafeSearchStrict   SafeSearchLevel = 2
)

// SearchRequest holds parsed user parameters and active options
type SearchRequest struct {
	Query             string
	RawQuery          string
	Category          Category
	Page              int
	TimeRange         string // "", "day", "week", "month", "year"
	SafeSearch        SafeSearchLevel
	Language          string
	EnabledEngines    []string
	Format            string // "html", "json", "rss", "csv"
	EnableRedirects   bool
	DOIResolver       string // "oadoi.org", "sci-hub.se", "unpaywall.org", "libgen.is"
	OpenInNewTab      bool
	InfiniteScroll    bool
	Theme             string
	DirectRedirectURL string   // Filled if bang is a direct jump (e.g. !yt!)
	SiteFilter        string   // e.g. site:github.com
	ExcludedSites     []string // e.g. -site:pinterest.com
	RemoveTrackers    bool     // SearXNG tracker_url_remover plugin (default true)
	Autocomplete      string   // "duckduckgo", "google", "brave", "bing", "wikipedia", "startpage", "qwant", "all", "off"
	Filetype          string   // e.g. "pdf", "epub", "docx", "zip", "mp3"
	Intitle           string   // e.g. "tutorial"
	Inurl             string   // e.g. "wiki"
	ExactPhrase       string   // e.g. "exact matched phrase"
}

// SearchResult represents a single item returned by any search engine
type SearchResult struct {
	Title         string            `json:"title"`
	URL           string            `json:"url"`
	PrettyURL     string            `json:"pretty_url"`
	Content       string            `json:"content"`
	Engine        string            `json:"engine"`
	Engines       []string          `json:"engines"`   // All engines that returned this result
	Positions     []int             `json:"positions"` // Ranks/positions in source engines
	Score         float64           `json:"score"`
	Category      Category          `json:"category"`
	Thumbnail     string            `json:"thumbnail,omitempty"`
	ImageURL      string            `json:"img_src,omitempty"`
	PublishedDate *time.Time        `json:"published_date,omitempty"`
	Author        string            `json:"author,omitempty"`
	CachedURL     string            `json:"cached_url,omitempty"`
	MagnetURL     string            `json:"magnet_url,omitempty"`
	VideoURL      string            `json:"video_url,omitempty"`
	AudioURL      string            `json:"audio_url,omitempty"`
	Duration      string            `json:"duration,omitempty"`
	Resolution    string            `json:"resolution,omitempty"`
	Seeders       int               `json:"seeders,omitempty"`
	Leechers      int               `json:"leechers,omitempty"`
	FileSize      string            `json:"filesize,omitempty"`
	Clusters      string            `json:"clusters,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
}

// InstantAnswer represents a rich widget card shown at the top of results
type InstantAnswer struct {
	Type        string            `json:"type"` // "calculator", "infobox", "weather", "crypto", "tools", "map"
	Title       string            `json:"title"`
	Value       string            `json:"value"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url,omitempty"`
	Thumbnail   string            `json:"thumbnail,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	EmbedHTML   string            `json:"embed_html,omitempty"`
}

// EngineInfo contains metadata about a search engine
type EngineInfo struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
	Categories  []Category `json:"categories"`
	DefaultOn   bool       `json:"default_on"`
	Weight      float64    `json:"weight"`
	About       string     `json:"about"`
	AvgPingMs   int64      `json:"avg_ping_ms,omitempty"`
	SuccessRate float64    `json:"success_rate,omitempty"`
}

// TopicCluster represents a semantic topic cluster for grouping search results
type TopicCluster struct {
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Count int    `json:"count"`
	Key   string `json:"key"`
}

// SearchResponse holds full aggregated search results for UI or API
type SearchResponse struct {
	Query               string            `json:"query"`
	Category            Category          `json:"category"`
	Page                int               `json:"page"`
	Pageno              int               `json:"pageno,omitempty"` // SearXNG official alias
	TotalPages          int               `json:"total_pages"`
	NumberOfResults     int               `json:"number_of_results"`
	SearchTimeMs        int64             `json:"search_time_ms"`
	Answers             []InstantAnswer   `json:"answers,omitempty"`
	Infoboxes           []InstantAnswer   `json:"infoboxes,omitempty"`
	InstantAnswers      []InstantAnswer   `json:"instant_answers,omitempty"`
	Results             []SearchResult    `json:"results"`
	Clusters            []TopicCluster    `json:"clusters,omitempty"`
	Suggestions         []string          `json:"suggestions,omitempty"`
	EnginesUsed         []string          `json:"engines_used"`
	UnresponsiveEngines []string          `json:"unresponsive_engines,omitempty"`
	Errors              map[string]string `json:"errors,omitempty"`
	Theme               string            `json:"theme,omitempty"`
	SafeSearch          SafeSearchLevel   `json:"safesearch,omitempty"`
}

// EngineStatItem tracks telemetry per engine
type EngineStatItem struct {
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	TotalCount   int64   `json:"total_count"`
	SuccessCount int64   `json:"success_count"`
	ErrorCount   int64   `json:"error_count"`
	AvgPingMs    int64   `json:"avg_ping_ms"`
	LastPingMs   int64   `json:"last_ping_ms"`
	SuccessRate  float64 `json:"success_rate"`
	LastActive   string  `json:"last_active"`
}

// SystemStats holds global server and engine statistics
type SystemStats struct {
	Uptime       string           `json:"uptime"`
	TotalQueries int64            `json:"total_queries"`
	EngineStats  []EngineStatItem `json:"engine_stats"`
}
