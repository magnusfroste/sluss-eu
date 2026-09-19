package registry

import (
	"errors"
	"testing"
	"time"
)

func testDefinition() Definition {
	return Definition{
		RegistryVersion: "test-v1",
		CreatedAt:       time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		Providers: []Provider{
			{ID: "provider-a", Name: "Provider A", Status: ProviderStatusActive},
			{ID: "provider-b", Name: "Provider B", Status: ProviderStatusActive},
			{ID: "provider-c", Name: "Provider C", Status: ProviderStatusDisabled},
		},
		Models: []Model{
			{
				ID:              "cheap-chat",
				ProviderID:      "provider-a",
				ProviderModelID: "a-cheap",
				Tier:            TierCheap,
				Capabilities:    Capabilities{Chat: true, Streaming: true},
				Cost:            CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 100000, OutputMicrosPerMillionToken: 300000},
				Enabled:         true,
				QualityScores:   map[string]float64{"simple": 0.7},
				Strengths:       []string{"cheap"},
			},
			{
				ID:              "balanced-tools",
				ProviderID:      "provider-a",
				ProviderModelID: "a-balanced",
				Tier:            TierBalanced,
				Capabilities:    Capabilities{Chat: true, Streaming: true, ToolCalls: true, JSONSchema: true},
				Enabled:         true,
			},
			{
				ID:              "disabled-vision",
				ProviderID:      "provider-b",
				ProviderModelID: "b-vision",
				Tier:            TierPremium,
				Capabilities:    Capabilities{Chat: true, Vision: true},
				Enabled:         false,
			},
			{
				ID:              "disabled-provider-chat",
				ProviderID:      "provider-c",
				ProviderModelID: "c-chat",
				Tier:            TierBalanced,
				Capabilities:    Capabilities{Chat: true, Streaming: true},
				Enabled:         true,
			},
		},
	}
}

func TestSnapshotLookups(t *testing.T) {
	snapshot, err := NewSnapshot(testDefinition())
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	tests := []struct {
		name string
		fn   func() bool
	}{
		{
			name: "model id",
			fn: func() bool {
				model, ok := snapshot.Model("cheap-chat")
				return ok && model.ProviderID == "provider-a"
			},
		},
		{
			name: "missing model",
			fn: func() bool {
				_, ok := snapshot.Model("missing")
				return !ok
			},
		},
		{
			name: "provider id",
			fn: func() bool {
				provider, ok := snapshot.Provider("provider-b")
				return ok && provider.Name == "Provider B"
			},
		},
		{
			name: "missing provider",
			fn: func() bool {
				_, ok := snapshot.Provider("missing")
				return !ok
			},
		},
		{
			name: "provider model mapping",
			fn: func() bool {
				model, ok := snapshot.ModelByProviderModelID("provider-a", "a-balanced")
				return ok && model.ID == "balanced-tools"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.fn() {
				t.Fatalf("lookup failed")
			}
		})
	}
}

func TestEnabledModelsWithCapabilities(t *testing.T) {
	snapshot, err := NewSnapshot(testDefinition())
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	tests := []struct {
		name     string
		required Capabilities
		wantIDs  []string
	}{
		{
			name:     "chat includes enabled chat models",
			required: Capabilities{Chat: true},
			wantIDs:  []string{"balanced-tools", "cheap-chat"},
		},
		{
			name:     "tool calls and json schema",
			required: Capabilities{ToolCalls: true, JSONSchema: true},
			wantIDs:  []string{"balanced-tools"},
		},
		{
			name:     "disabled model filtered",
			required: Capabilities{Vision: true},
			wantIDs:  nil,
		},
		{
			name:     "disabled provider model filtered",
			required: Capabilities{Chat: true},
			wantIDs:  []string{"balanced-tools", "cheap-chat"},
		},
		{
			name:     "missing one capability filtered",
			required: Capabilities{Streaming: true, Vision: true},
			wantIDs:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models := snapshot.EnabledModelsWithCapabilities(tt.required)
			got := make([]string, 0, len(models))
			for _, model := range models {
				got = append(got, model.ID)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %v, want %v", got, tt.wantIDs)
			}
			for i := range got {
				if got[i] != tt.wantIDs[i] {
					t.Fatalf("got %v, want %v", got, tt.wantIDs)
				}
			}
		})
	}
}

func TestStoreReloadSwapsOnlyValidSnapshots(t *testing.T) {
	initial, err := NewSnapshot(testDefinition())
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	store, err := NewStore(initial)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	nextDef := testDefinition()
	nextDef.RegistryVersion = "test-v2"
	if _, err := store.Reload(nextDef); err != nil {
		t.Fatalf("Reload valid: %v", err)
	}
	active, err := store.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active.RegistryVersion() != "test-v2" {
		t.Fatalf("got version %q", active.RegistryVersion())
	}
	badDef := testDefinition()
	badDef.RegistryVersion = "bad-v3"
	badDef.Models[0].ProviderID = "missing-provider"
	if _, err := store.Reload(badDef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected invalid snapshot error, got %v", err)
	}
	active, err = store.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active.RegistryVersion() != "test-v2" {
		t.Fatalf("failed reload should preserve test-v2, got %q", active.RegistryVersion())
	}
}

func TestSnapshotReadersCannotMutateSharedState(t *testing.T) {
	snapshot, err := NewSnapshot(testDefinition())
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	model, ok := snapshot.Model("cheap-chat")
	if !ok {
		t.Fatalf("missing model")
	}
	model.Strengths[0] = "mutated"
	model.QualityScores["simple"] = 0.1
	model.ProviderModelID = "mutated"

	again, ok := snapshot.Model("cheap-chat")
	if !ok {
		t.Fatalf("missing model on second read")
	}
	if again.Strengths[0] != "cheap" {
		t.Fatalf("shared strengths slice mutated: %+v", again.Strengths)
	}
	if again.QualityScores["simple"] != 0.7 {
		t.Fatalf("shared quality map mutated: %+v", again.QualityScores)
	}
	if again.ProviderModelID != "a-cheap" {
		t.Fatalf("shared model value mutated: %+v", again)
	}
}
