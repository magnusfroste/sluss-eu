package registry

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrNoActiveSnapshot = errors.New("registry: no active snapshot")
	ErrInvalidSnapshot  = errors.New("registry: invalid snapshot")
)

type Snapshot struct {
	registryVersion  string
	createdAt        time.Time
	providers        map[string]Provider
	models           map[string]Model
	modelsByProvider map[string][]string
	providerModelIDs map[string]string
}

func NewSnapshot(def Definition) (*Snapshot, error) {
	if strings.TrimSpace(def.RegistryVersion) == "" {
		return nil, fmt.Errorf("%w: registry version is required", ErrInvalidSnapshot)
	}
	if len(def.Providers) == 0 {
		return nil, fmt.Errorf("%w: at least one provider is required", ErrInvalidSnapshot)
	}
	if len(def.Models) == 0 {
		return nil, fmt.Errorf("%w: at least one model is required", ErrInvalidSnapshot)
	}
	createdAt := def.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	s := &Snapshot{
		registryVersion:  def.RegistryVersion,
		createdAt:        createdAt.UTC(),
		providers:        make(map[string]Provider, len(def.Providers)),
		models:           make(map[string]Model, len(def.Models)),
		modelsByProvider: make(map[string][]string),
		providerModelIDs: make(map[string]string, len(def.Models)),
	}
	for _, provider := range def.Providers {
		if strings.TrimSpace(provider.ID) == "" {
			return nil, fmt.Errorf("%w: provider id is required", ErrInvalidSnapshot)
		}
		if _, exists := s.providers[provider.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate provider %q", ErrInvalidSnapshot, provider.ID)
		}
		s.providers[provider.ID] = provider
	}
	for _, model := range def.Models {
		if strings.TrimSpace(model.ID) == "" {
			return nil, fmt.Errorf("%w: model id is required", ErrInvalidSnapshot)
		}
		if _, exists := s.models[model.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate model %q", ErrInvalidSnapshot, model.ID)
		}
		if strings.TrimSpace(model.ProviderID) == "" {
			return nil, fmt.Errorf("%w: model %q missing provider id", ErrInvalidSnapshot, model.ID)
		}
		if _, exists := s.providers[model.ProviderID]; !exists {
			return nil, fmt.Errorf("%w: model %q references missing provider %q", ErrInvalidSnapshot, model.ID, model.ProviderID)
		}
		if strings.TrimSpace(model.ProviderModelID) == "" {
			return nil, fmt.Errorf("%w: model %q missing provider model id", ErrInvalidSnapshot, model.ID)
		}
		if model.ContextWindowTokens < 0 {
			return nil, fmt.Errorf("%w: model %q has negative context window", ErrInvalidSnapshot, model.ID)
		}
		key := providerModelKey(model.ProviderID, model.ProviderModelID)
		if existing := s.providerModelIDs[key]; existing != "" {
			return nil, fmt.Errorf("%w: provider model %q/%q used by %q and %q", ErrInvalidSnapshot, model.ProviderID, model.ProviderModelID, existing, model.ID)
		}
		// Materialize effective compliance tags: the model inherits its
		// provider's tags and may add its own (union, sorted, deduped) — so the
		// engine's constraint filtering reads a single field (ISSUE-078).
		model.ComplianceTags = unionTags(s.providers[model.ProviderID].ComplianceTags, model.ComplianceTags)
		s.models[model.ID] = cloneModel(model)
		s.modelsByProvider[model.ProviderID] = append(s.modelsByProvider[model.ProviderID], model.ID)
		s.providerModelIDs[key] = model.ID
	}
	for providerID := range s.modelsByProvider {
		sort.Strings(s.modelsByProvider[providerID])
	}
	return s, nil
}

func (s *Snapshot) RegistryVersion() string {
	if s == nil {
		return ""
	}
	return s.registryVersion
}

func (s *Snapshot) CreatedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.createdAt
}

func (s *Snapshot) Model(id string) (Model, bool) {
	if s == nil {
		return Model{}, false
	}
	model, ok := s.models[id]
	return cloneModel(model), ok
}

func (s *Snapshot) Provider(id string) (Provider, bool) {
	if s == nil {
		return Provider{}, false
	}
	provider, ok := s.providers[id]
	return provider, ok
}

// Providers returns all providers in the snapshot, sorted by ID.
func (s *Snapshot) Providers() []Provider {
	if s == nil {
		return nil
	}
	out := make([]Provider, 0, len(s.providers))
	for _, p := range s.providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Snapshot) ModelByProviderModelID(providerID, providerModelID string) (Model, bool) {
	if s == nil {
		return Model{}, false
	}
	id := s.providerModelIDs[providerModelKey(providerID, providerModelID)]
	if id == "" {
		return Model{}, false
	}
	return s.Model(id)
}

func (s *Snapshot) ModelsForProvider(providerID string) []Model {
	if s == nil {
		return nil
	}
	ids := s.modelsByProvider[providerID]
	models := make([]Model, 0, len(ids))
	for _, id := range ids {
		models = append(models, cloneModel(s.models[id]))
	}
	return models
}

func (s *Snapshot) EnabledModelsWithCapabilities(required Capabilities) []Model {
	if s == nil {
		return nil
	}
	ids := make([]string, 0, len(s.models))
	for id := range s.models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	models := make([]Model, 0, len(ids))
	for _, id := range ids {
		model := s.models[id]
		if s.modelSelectable(model) && model.Capabilities.Satisfies(required) {
			models = append(models, cloneModel(model))
		}
	}
	return models
}

func (s *Snapshot) ModelSelectable(model Model) bool {
	if s == nil {
		return false
	}
	return s.modelSelectable(model)
}

func (s *Snapshot) modelSelectable(model Model) bool {
	provider, ok := s.providers[model.ProviderID]
	return ok && provider.Status == ProviderStatusActive && model.Enabled
}

type Store struct {
	active atomic.Value
}

func NewStore(initial *Snapshot) (*Store, error) {
	if initial == nil {
		return nil, ErrNoActiveSnapshot
	}
	store := &Store{}
	store.active.Store(initial)
	return store, nil
}

func (s *Store) Active() (*Snapshot, error) {
	if s == nil {
		return nil, ErrNoActiveSnapshot
	}
	active, ok := s.active.Load().(*Snapshot)
	if !ok || active == nil {
		return nil, ErrNoActiveSnapshot
	}
	return active, nil
}

func (s *Store) Reload(candidate Definition) (*Snapshot, error) {
	if s == nil {
		return nil, ErrNoActiveSnapshot
	}
	next, err := NewSnapshot(candidate)
	if err != nil {
		return nil, err
	}
	s.active.Store(next)
	return next, nil
}

func providerModelKey(providerID, providerModelID string) string {
	return providerID + "\x00" + providerModelID
}

func cloneModel(model Model) Model {
	model.Strengths = append([]string(nil), model.Strengths...)
	model.Weaknesses = append([]string(nil), model.Weaknesses...)
	model.ComplianceTags = append([]string(nil), model.ComplianceTags...)
	if model.QualityScores != nil {
		model.QualityScores = cloneQualityScores(model.QualityScores)
	}
	return model
}

// unionTags merges two tag sets into a sorted, deduped, lowercased slice.
// Returns nil when both are empty.
func unionTags(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, list := range [2][]string{a, b} {
		for _, tag := range list {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	sort.Strings(out)
	return out
}

func cloneQualityScores(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
