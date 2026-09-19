package profiles

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

func profileTestSnapshot(t *testing.T, models []registry.Model) *registry.Snapshot {
	t.Helper()
	snapshot, err := registry.NewSnapshot(registry.Definition{
		RegistryVersion: "profiles-test",
		Providers: []registry.Provider{
			{ID: "provider-a", Name: "Provider A", Status: registry.ProviderStatusActive},
			{ID: "provider-disabled", Name: "Disabled Provider", Status: registry.ProviderStatusDisabled},
		},
		Models: models,
	})
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	return snapshot
}

func profileTestModels() []registry.Model {
	return []registry.Model{
		{
			ID:              "cheap-general",
			ProviderID:      "provider-a",
			ProviderModelID: "a-cheap",
			Tier:            registry.TierCheap,
			Capabilities:    registry.Capabilities{Chat: true, Streaming: true},
			Enabled:         true,
		},
		{
			ID:              "balanced-coder",
			ProviderID:      "provider-a",
			ProviderModelID: "a-balanced",
			Tier:            registry.TierBalanced,
			Capabilities:    registry.Capabilities{Chat: true, ToolCalls: true, JSONSchema: true},
			Enabled:         true,
		},
		{
			ID:              "premium-reasoning",
			ProviderID:      "provider-a",
			ProviderModelID: "a-premium",
			Tier:            registry.TierPremium,
			Capabilities:    registry.Capabilities{Chat: true, ToolCalls: true},
			Enabled:         true,
		},
	}
}

func TestResolveProfilesAndTiers(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog: %v", err)
	}
	snapshot := profileTestSnapshot(t, profileTestModels())
	tests := []struct {
		name      string
		selectr   Selector
		wantID    ID
		wantModel string
	}{
		{name: "cheap tier", selectr: Selector{Tier: registry.TierCheap}, wantID: IDCheapGeneral, wantModel: "cheap-general"},
		{name: "balanced tier", selectr: Selector{Tier: registry.TierBalanced}, wantID: IDBalancedCoder, wantModel: "balanced-coder"},
		{name: "premium tier", selectr: Selector{Tier: registry.TierPremium}, wantID: IDPremiumReasoning, wantModel: "premium-reasoning"},
		{name: "named cheap", selectr: Selector{ProfileID: IDCheapGeneral}, wantID: IDCheapGeneral, wantModel: "cheap-general"},
		{name: "named coder", selectr: Selector{ProfileID: IDBalancedCoder}, wantID: IDBalancedCoder, wantModel: "balanced-coder"},
		{name: "named reasoning", selectr: Selector{ProfileID: IDPremiumReasoning}, wantID: IDPremiumReasoning, wantModel: "premium-reasoning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := catalog.Resolve(snapshot, tt.selectr)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.ProfileID != tt.wantID || len(got.Models) != 1 || got.Models[0].ID != tt.wantModel {
				t.Fatalf("got %+v", got)
			}
			if got.Models[0].ProviderModelID == string(tt.wantID) {
				t.Fatalf("profile resolution leaked provider model id into policy identity")
			}
		})
	}
}

func TestResolveRejectsMissingDisabledAndCapabilityFailures(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog: %v", err)
	}
	tests := []struct {
		name    string
		models  []registry.Model
		selectr Selector
		wantErr error
	}{
		{
			name:    "missing profile",
			models:  profileTestModels(),
			selectr: Selector{ProfileID: "unknown"},
			wantErr: ErrMissingProfile,
		},
		{
			name: "disabled target model",
			models: []registry.Model{
				{
					ID:              "cheap-general",
					ProviderID:      "provider-a",
					ProviderModelID: "a-cheap",
					Tier:            registry.TierCheap,
					Capabilities:    registry.Capabilities{Chat: true},
					Enabled:         false,
				},
			},
			selectr: Selector{ProfileID: IDCheapGeneral},
			wantErr: ErrNoEnabledTargetModel,
		},
		{
			name: "disabled provider target model",
			models: []registry.Model{
				{
					ID:              "cheap-general",
					ProviderID:      "provider-disabled",
					ProviderModelID: "disabled-cheap",
					Tier:            registry.TierCheap,
					Capabilities:    registry.Capabilities{Chat: true},
					Enabled:         true,
				},
			},
			selectr: Selector{ProfileID: IDCheapGeneral},
			wantErr: ErrNoEnabledTargetModel,
		},
		{
			name: "required capability missing",
			models: []registry.Model{
				{
					ID:              "premium-reasoning",
					ProviderID:      "provider-a",
					ProviderModelID: "a-premium",
					Tier:            registry.TierPremium,
					Capabilities:    registry.Capabilities{Chat: true},
					Enabled:         true,
				},
			},
			selectr: Selector{ProfileID: IDPremiumReasoning, RequiredCapabilities: registry.Capabilities{JSONSchema: true}},
			wantErr: ErrMissingCapabilities,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := profileTestSnapshot(t, tt.models)
			_, err := catalog.Resolve(snapshot, tt.selectr)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
		})
	}
}
