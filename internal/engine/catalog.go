package engine

import (
	"context"

	"searxgo/internal/models"
)

// EngineDefinition represents full metadata for an engine matching priv.au / SearXNG
type EngineDefinition struct {
	ID                 string          `json:"id"`
	DisplayName        string          `json:"name"`
	Category           models.Category `json:"category"`
	Group              string          `json:"group"`
	Shortcut           string          `json:"shortcut"`
	Homepage           string          `json:"homepage"`
	DefaultEnabled     bool            `json:"enabled"`
	SupportsSafeSearch bool            `json:"supports_safesearch"`
	SupportsTimeRange  bool            `json:"supports_timerange"`
	Weight             float64         `json:"weight"`
	MaxTime            float64         `json:"max_time"`
	About              string          `json:"about"`
}

// EngineGroup represents a grouped section within a category (e.g. blogs, books, web)
type EngineGroup struct {
	Name    string
	Bang    string
	Engines []EngineDefinition
}

// CategoryEngineGroup represents a top-level category containing grouped engines
type CategoryEngineGroup struct {
	Category string
	Groups   []EngineGroup
}

// FullEngineCatalog contains all 260 engines from priv.au / SearXNG
// FullEngineCatalog aggregates all engines across catalog modules (General, Media, Dev/Sci, Files/Other)
var FullEngineCatalog = func() []EngineDefinition {
	all := make([]EngineDefinition, 0, len(generalEngines)+len(mediaEngines)+len(devScienceEngines)+len(filesOtherEngines))
	all = append(all, generalEngines...)
	all = append(all, mediaEngines...)
	all = append(all, devScienceEngines...)
	all = append(all, filesOtherEngines...)
	return all
}()

// GetCategorizedCatalog returns all engines organized by Category and Sub-groups
func GetCategorizedCatalog() []CategoryEngineGroup {
	categories := []string{"general", "images", "videos", "news", "music", "it", "science", "files", "social", "maps", "other"}
	var result []CategoryEngineGroup

	for _, cat := range categories {
		groupMap := make(map[string][]EngineDefinition)
		var groupOrder []string

		for _, def := range FullEngineCatalog {
			if string(def.Category) == cat {
				grp := def.Group
				if grp == "" {
					grp = "without further subgrouping"
				}
				if _, exists := groupMap[grp]; !exists {
					groupOrder = append(groupOrder, grp)
				}
				groupMap[grp] = append(groupMap[grp], def)
			}
		}

		var groups []EngineGroup
		for _, grpName := range groupOrder {
			bang := "!" + grpName
			if grpName == "without further subgrouping" {
				bang = ""
			} else if grpName == "social media" {
				bang = "!social_media"
			} else if grpName == "scientific publications" {
				bang = "!scientific_publications"
			} else if grpName == "software wikis" {
				bang = "!software_wikis"
			}
			groups = append(groups, EngineGroup{
				Name:    grpName,
				Bang:    bang,
				Engines: groupMap[grpName],
			})
		}

		result = append(result, CategoryEngineGroup{
			Category: cat,
			Groups:   groups,
		})
	}

	return result
}

// GenericCatalogEngine wraps any catalog engine with live search capabilities
type GenericCatalogEngine struct {
	def EngineDefinition
}

func NewGenericCatalogEngine(def EngineDefinition) *GenericCatalogEngine {
	return &GenericCatalogEngine{def: def}
}

func (e *GenericCatalogEngine) Name() string {
	return e.def.ID
}

func (e *GenericCatalogEngine) DisplayName() string {
	return e.def.DisplayName
}

func (e *GenericCatalogEngine) Categories() []models.Category {
	return []models.Category{e.def.Category}
}

func (e *GenericCatalogEngine) DefaultOn() bool {
	return e.def.DefaultEnabled
}

func (e *GenericCatalogEngine) Weight() float64 {
	return e.def.Weight
}

func (e *GenericCatalogEngine) About() string {
	return e.def.About
}

func (e *GenericCatalogEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// Execute specialized scraper if available, or fast live search adapter
	return executeCatalogEngineSearch(ctx, e.def, req)
}

// RegisterAllCatalogEngines registers all 260 engines into the global registry
func RegisterAllCatalogEngines(r *Registry) {
	for _, def := range FullEngineCatalog {
		// If an engine already has a custom implementation, don't overwrite it, but update metadata
		if _, exists := r.GetByName(def.ID); !exists {
			r.Register(NewGenericCatalogEngine(def))
		}
	}
}
