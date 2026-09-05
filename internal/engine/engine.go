package engine

import (
	"context"
	"sync"

	"searxgo/internal/models"
)

// Engine defines the contract for any search engine integration
type Engine interface {
	Name() string
	DisplayName() string
	Categories() []models.Category
	DefaultOn() bool
	Weight() float64
	About() string
	Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error)
}

// Registry manages all available search engines
type Registry struct {
	mu      sync.RWMutex
	engines map[string]Engine
	order   []string
}

// NewRegistry creates an empty engine registry
func NewRegistry() *Registry {
	return &Registry{
		engines: make(map[string]Engine),
		order:   make([]string, 0),
	}
}

// Register adds an engine to the registry
func (r *Registry) Register(e Engine) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := e.Name()
	if _, exists := r.engines[name]; !exists {
		r.order = append(r.order, name)
	}
	r.engines[name] = e
}

// GetAll returns all registered engines in registration order
func (r *Registry) GetAll() []Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Engine, 0, len(r.order))
	for _, name := range r.order {
		list = append(list, r.engines[name])
	}
	return list
}

// GetByName retrieves a specific engine by its identifier
func (r *Registry) GetByName(name string) (Engine, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.engines[name]
	return e, ok
}

// GetByCategory returns engines that support the specified category
func (r *Registry) GetByCategory(cat models.Category) []Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []Engine
	for _, name := range r.order {
		e := r.engines[name]
		for _, c := range e.Categories() {
			if c == cat {
				matched = append(matched, e)
				break
			}
		}
	}
	return matched
}

// GetEngineInfos returns summary metadata for all engines
func (r *Registry) GetEngineInfos() []models.EngineInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]models.EngineInfo, 0, len(r.order))
	for _, name := range r.order {
		e := r.engines[name]
		infos = append(infos, models.EngineInfo{
			Name:        e.Name(),
			DisplayName: e.DisplayName(),
			Categories:  e.Categories(),
			DefaultOn:   e.DefaultOn(),
			Weight:      e.Weight(),
			About:       e.About(),
		})
	}
	return infos
}

// DefaultRegistry is the global engine registry
var DefaultRegistry = NewRegistry()
