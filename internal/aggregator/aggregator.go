package aggregator

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"searxgo/internal/bangs"
	"searxgo/internal/engine"
	"searxgo/internal/models"
	"searxgo/internal/plugins"
	"searxgo/internal/stats"
)

type Aggregator struct {
	registry *engine.Registry
	timeout  time.Duration
}

func NewAggregator(registry *engine.Registry, timeout time.Duration) *Aggregator {
	return &Aggregator{
		registry: registry,
		timeout:  timeout,
	}
}

// Search executes the query across all matching and enabled engines concurrently
func (a *Aggregator) Search(ctx context.Context, req models.SearchRequest) (*models.SearchResponse, error) {
	startTime := time.Now()
	stats.GlobalTracker.RecordQuery()

	// Default category to general if empty
	if req.Category == "" {
		req.Category = models.CategoryGeneral
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	// 1. Parse Bangs and Query Modifiers
	bangParsed := bangs.ParseBangs(req.Query, req.Category)
	req.Query = bangParsed.CleanQuery
	req.Category = bangParsed.Category
	if bangParsed.Language != "" {
		req.Language = bangParsed.Language
	}
	if bangParsed.SiteFilter != "" {
		req.SiteFilter = bangParsed.SiteFilter
	}
	if len(bangParsed.ExcludedSites) > 0 {
		req.ExcludedSites = bangParsed.ExcludedSites
	}
	if bangParsed.Filetype != "" {
		req.Filetype = bangParsed.Filetype
	}
	if bangParsed.Intitle != "" {
		req.Intitle = bangParsed.Intitle
	}
	if bangParsed.Inurl != "" {
		req.Inurl = bangParsed.Inurl
	}
	if bangParsed.ExactPhrase != "" {
		req.ExactPhrase = bangParsed.ExactPhrase
	}

	// If direct redirect bang was used (e.g. !yt! lo-fi), return early with redirect URL
	if bangParsed.DirectRedirectURL != "" {
		return &models.SearchResponse{
			Query:       req.Query,
			Category:    req.Category,
			Results:     []models.SearchResult{},
			EnginesUsed: []string{},
		}, nil
	}

	// 2. Determine candidate engines
	var availableEngines []engine.Engine
	if len(bangParsed.Engines) > 0 {
		// Specific bang forced: e.g. !gh, !so, !yt
		for _, name := range bangParsed.Engines {
			if e, ok := a.registry.GetByName(name); ok {
				availableEngines = append(availableEngines, e)
			}
		}
	} else {
		availableEngines = a.registry.GetByCategory(req.Category)
	}

	// 3. Filter by user's enabled engines if provided
	var selectedEngines []engine.Engine
	if len(req.EnabledEngines) > 0 && len(bangParsed.Engines) == 0 {
		enabledMap := make(map[string]bool)
		for _, name := range req.EnabledEngines {
			enabledMap[strings.ToLower(strings.TrimSpace(name))] = true
		}
		for _, e := range availableEngines {
			if enabledMap[strings.ToLower(e.Name())] {
				selectedEngines = append(selectedEngines, e)
			}
		}
	} else if len(bangParsed.Engines) == 0 {
		// Default behavior (SearXNG Parity): Query engines enabled by default for this category
		for _, e := range availableEngines {
			if e.DefaultOn() {
				selectedEngines = append(selectedEngines, e)
			}
		}
	} else {
		// Bang specified engines
		selectedEngines = availableEngines
	}

	// 3.1 Apply engine exclusions (e.g. -!google or !~bing)
	if len(bangParsed.ExcludedEngines) > 0 {
		excludeMap := make(map[string]bool)
		for _, ex := range bangParsed.ExcludedEngines {
			excludeMap[strings.ToLower(ex)] = true
		}
		var filtered []engine.Engine
		for _, e := range selectedEngines {
			if !excludeMap[strings.ToLower(e.Name())] {
				filtered = append(filtered, e)
			}
		}
		selectedEngines = filtered
	}

	// If no engines selected, fallback to all category engines
	if len(selectedEngines) == 0 {
		selectedEngines = availableEngines
	}

	// 4. Execute searches concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex

	var allRawResults []models.SearchResult
	errorsMap := make(map[string]string)
	enginesUsed := make([]string, 0, len(selectedEngines))
	unresponsiveEngines := make([]string, 0)

	// Determine effective timeout (honoring query <timeout> modifier if set)
	effectiveTimeout := a.timeout
	if bangParsed.TimeoutLimit > 0 {
		effectiveTimeout = bangParsed.TimeoutLimit
	}

	// Timeout context for overall engine search
	searchCtx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	for _, eng := range selectedEngines {
		wg.Add(1)
		go func(e engine.Engine) {
			defer wg.Done()
			eStart := time.Now()

			res, err := e.Search(searchCtx, req)
			pingMs := time.Since(eStart).Milliseconds()

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				stats.GlobalTracker.RecordEngineResult(e.Name(), e.DisplayName(), pingMs, false)
				errorsMap[e.Name()] = err.Error()
				unresponsiveEngines = append(unresponsiveEngines, e.Name())
				return
			}

			stats.GlobalTracker.RecordEngineResult(e.Name(), e.DisplayName(), pingMs, true)

			if len(res) > 0 {
				enginesUsed = append(enginesUsed, e.Name())
				// Score initial results with engine weight
				scored := CalculateInitialScores(res, e.Weight())
				allRawResults = append(allRawResults, scored...)
			}
		}(eng)
	}

	wg.Wait()

	// 5. Deduplicate and merge results
	deduped := Deduplicate(allRawResults)

	// 5.1 Filter by site: and -site: if specified
	if req.SiteFilter != "" || len(req.ExcludedSites) > 0 {
		var siteFiltered []models.SearchResult
		for _, item := range deduped {
			u, err := url.Parse(item.URL)
			if err != nil {
				siteFiltered = append(siteFiltered, item)
				continue
			}
			host := strings.ToLower(u.Host)
			host = strings.TrimPrefix(host, "www.")

			// Check exclusion
			isExcluded := false
			for _, ex := range req.ExcludedSites {
				if host == ex || strings.HasSuffix(host, "."+ex) {
					isExcluded = true
					break
				}
			}
			if isExcluded {
				continue
			}

			// Check inclusion
			if req.SiteFilter != "" {
				if host != req.SiteFilter && !strings.HasSuffix(host, "."+req.SiteFilter) {
					continue
				}
			}

			siteFiltered = append(siteFiltered, item)
		}
		deduped = siteFiltered
	}

	// 5.2 Filter by filetype: / ext: if specified (e.g. filetype:pdf)
	if req.Filetype != "" {
		targetExt := strings.ToLower(strings.TrimPrefix(req.Filetype, "."))
		var filetypeFiltered []models.SearchResult
		for _, item := range deduped {
			u, err := url.Parse(item.URL)
			if err == nil {
				pathLower := strings.ToLower(u.Path)
				if strings.HasSuffix(pathLower, "."+targetExt) || strings.Contains(strings.ToLower(item.URL), "."+targetExt) || strings.Contains(strings.ToLower(item.Title), "."+targetExt) || strings.Contains(strings.ToLower(item.Content), targetExt) {
					filetypeFiltered = append(filetypeFiltered, item)
					continue
				}
			}
		}
		if len(filetypeFiltered) > 0 {
			deduped = filetypeFiltered
		}
	}

	// 5.3 Filter by intitle: if specified (e.g. intitle:tutorial)
	if req.Intitle != "" {
		intitleLower := strings.ToLower(req.Intitle)
		var intitleFiltered []models.SearchResult
		for _, item := range deduped {
			if strings.Contains(strings.ToLower(item.Title), intitleLower) {
				intitleFiltered = append(intitleFiltered, item)
			}
		}
		if len(intitleFiltered) > 0 {
			deduped = intitleFiltered
		}
	}

	// 5.4 Filter by inurl: if specified (e.g. inurl:docs)
	if req.Inurl != "" {
		inurlLower := strings.ToLower(req.Inurl)
		var inurlFiltered []models.SearchResult
		for _, item := range deduped {
			if strings.Contains(strings.ToLower(item.URL), inurlLower) {
				inurlFiltered = append(inurlFiltered, item)
			}
		}
		if len(inurlFiltered) > 0 {
			deduped = inurlFiltered
		}
	}

	// 6. Apply Search Result Plugins (Privacy Rewriters, DOI Resolvers, Tracker Remover, Wayback Cached)
	processed := plugins.ProcessSearchResultPlugins(deduped, req.EnableRedirects, req.DOIResolver, req.RemoveTrackers)

	// 7. Rank and Sort
	ranked := RankAndSort(processed, req.Query)

	// 7.1 Generate Smart Topic Clusters (Semantic result categorisation)
	clusteredResults, clusters := GenerateTopicClusters(ranked, req.Query)
	ranked = clusteredResults

	// 8. Pagination (SearXNG Unlimited Search & max_page: 0 Parity)
	// Engines are queried upstream with the offset corresponding to req.Page.
	// Therefore, the deduplicated and ranked results represent the current page pool.
	pageSize := 15
	totalResults := len(ranked)
	var pagedResults []models.SearchResult
	if len(ranked) > pageSize {
		pagedResults = ranked[:pageSize]
	} else {
		pagedResults = ranked
	}

	// For Unlimited Search: if current page yielded results, next page is always accessible.
	totalPages := req.Page
	if len(ranked) > 0 {
		totalPages = req.Page + 1
	}
	if totalPages < 1 {
		totalPages = 1
	}

	elapsed := time.Since(startTime).Milliseconds()

	return &models.SearchResponse{
		Query:               req.Query,
		Category:            req.Category,
		Page:                req.Page,
		Pageno:              req.Page,
		TotalPages:          totalPages,
		NumberOfResults:     totalResults,
		SearchTimeMs:        elapsed,
		Results:             pagedResults,
		Clusters:            clusters,
		EnginesUsed:         enginesUsed,
		UnresponsiveEngines: unresponsiveEngines,
		Errors:              errorsMap,
		SafeSearch:          req.SafeSearch,
	}, nil
}
