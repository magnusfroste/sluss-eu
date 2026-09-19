// Package profiles resolves stable policy-facing model profile ids to
// registry model ids.
package profiles

import (
	"errors"
	"fmt"
	"sort"

	"github.com/magnusfroste/sluss/internal/registry"
)

type ID string

const (
	IDCheapGeneral     ID = "cheap-general"
	IDBalancedCoder    ID = "balanced-coder"
	IDPremiumReasoning ID = "premium-reasoning"
)

var (
	ErrMissingProfile       = errors.New("profiles: missing profile")
	ErrNoEnabledTargetModel = errors.New("profiles: no enabled target model")
	ErrMissingCapabilities  = errors.New("profiles: target models missing required capabilities")
)

type Profile struct {
	ID                   ID
	Tier                 registry.Tier
	ModelIDs             []string
	RequiredCapabilities registry.Capabilities
	IntendedTaskClasses  []string
}

type Catalog struct {
	profiles map[ID]Profile
	tiers    map[registry.Tier][]ID
}

type Selector struct {
	ProfileID            ID
	Tier                 registry.Tier
	RequiredCapabilities registry.Capabilities
}

type Resolution struct {
	ProfileID            ID
	Tier                 registry.Tier
	RequiredCapabilities registry.Capabilities
	Models               []registry.Model
}

func NewCatalog(profiles []Profile) (*Catalog, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("%w: no profiles configured", ErrMissingProfile)
	}
	catalog := &Catalog{
		profiles: make(map[ID]Profile, len(profiles)),
		tiers:    make(map[registry.Tier][]ID),
	}
	for _, profile := range profiles {
		if profile.ID == "" {
			return nil, fmt.Errorf("%w: profile id is required", ErrMissingProfile)
		}
		if len(profile.ModelIDs) == 0 {
			return nil, fmt.Errorf("%w: profile %q has no target models", ErrNoEnabledTargetModel, profile.ID)
		}
		if _, exists := catalog.profiles[profile.ID]; exists {
			return nil, fmt.Errorf("profiles: duplicate profile %q", profile.ID)
		}
		catalog.profiles[profile.ID] = cloneProfile(profile)
		catalog.tiers[profile.Tier] = append(catalog.tiers[profile.Tier], profile.ID)
	}
	for tier := range catalog.tiers {
		sort.Slice(catalog.tiers[tier], func(i, j int) bool {
			return catalog.tiers[tier][i] < catalog.tiers[tier][j]
		})
	}
	return catalog, nil
}

func DefaultCatalog() (*Catalog, error) {
	return NewCatalog(DefaultProfiles())
}

func DefaultProfiles() []Profile {
	return []Profile{
		{
			ID:       IDCheapGeneral,
			Tier:     registry.TierCheap,
			ModelIDs: []string{"cheap-general"},
			RequiredCapabilities: registry.Capabilities{
				Chat: true,
			},
			IntendedTaskClasses: []string{"simple", "summarization", "low_risk"},
		},
		{
			ID:       IDBalancedCoder,
			Tier:     registry.TierBalanced,
			ModelIDs: []string{"balanced-coder"},
			RequiredCapabilities: registry.Capabilities{
				Chat:       true,
				ToolCalls:  true,
				JSONSchema: true,
			},
			IntendedTaskClasses: []string{"code", "analysis", "tool_use"},
		},
		{
			ID:       IDPremiumReasoning,
			Tier:     registry.TierPremium,
			ModelIDs: []string{"premium-reasoning"},
			RequiredCapabilities: registry.Capabilities{
				Chat: true,
			},
			IntendedTaskClasses: []string{"hard_reasoning", "security_review", "high_risk"},
		},
	}
}

func (c *Catalog) Profile(id ID) (Profile, bool) {
	if c == nil {
		return Profile{}, false
	}
	profile, ok := c.profiles[id]
	return cloneProfile(profile), ok
}

func (c *Catalog) Resolve(snapshot *registry.Snapshot, selector Selector) (Resolution, error) {
	if snapshot == nil {
		return Resolution{}, registry.ErrNoActiveSnapshot
	}
	profileIDs, err := c.matchingProfileIDs(selector)
	if err != nil {
		return Resolution{}, err
	}
	var firstMissingCapability bool
	for _, profileID := range profileIDs {
		profile := c.profiles[profileID]
		required := profile.RequiredCapabilities.Merge(selector.RequiredCapabilities)
		models := make([]registry.Model, 0, len(profile.ModelIDs))
		missingCapability := false
		for _, modelID := range profile.ModelIDs {
			model, ok := snapshot.Model(modelID)
			if !ok || !snapshot.ModelSelectable(model) {
				continue
			}
			if !model.Capabilities.Satisfies(required) {
				missingCapability = true
				continue
			}
			models = append(models, model)
		}
		if len(models) > 0 {
			return Resolution{
				ProfileID:            profile.ID,
				Tier:                 profile.Tier,
				RequiredCapabilities: required,
				Models:               models,
			}, nil
		}
		firstMissingCapability = firstMissingCapability || missingCapability
	}
	if firstMissingCapability {
		return Resolution{}, ErrMissingCapabilities
	}
	return Resolution{}, ErrNoEnabledTargetModel
}

func (c *Catalog) matchingProfileIDs(selector Selector) ([]ID, error) {
	if c == nil {
		return nil, ErrMissingProfile
	}
	if selector.ProfileID != "" {
		if _, ok := c.profiles[selector.ProfileID]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingProfile, selector.ProfileID)
		}
		return []ID{selector.ProfileID}, nil
	}
	if selector.Tier == "" {
		return nil, ErrMissingProfile
	}
	ids := c.tiers[selector.Tier]
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: tier %q", ErrMissingProfile, selector.Tier)
	}
	return append([]ID(nil), ids...), nil
}

func cloneProfile(profile Profile) Profile {
	profile.ModelIDs = append([]string(nil), profile.ModelIDs...)
	profile.IntendedTaskClasses = append([]string(nil), profile.IntendedTaskClasses...)
	return profile
}
