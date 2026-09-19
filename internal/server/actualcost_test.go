package server

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
)

func TestActualCostUSDFromRegistry(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	cfg := &ChatOptions{Engine: engine.New(store)}

	// cheap-general: $0.40/Mtok input, $1.60/Mtok output → 1M+1M = $2.00.
	got := cfg.actualCostUSD("cheap-general", "openai", 1_000_000, 1_000_000)
	if got != 2.0 {
		t.Fatalf("actual cost = %v, want 2.0", got)
	}
}

func TestActualCostUSDUnknownModelIsZero(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	store, _ := registry.NewStore(snap)
	cfg := &ChatOptions{Engine: engine.New(store)}
	if got := cfg.actualCostUSD("does-not-exist", "openai", 100, 100); got != 0 {
		t.Errorf("unknown model cost = %v, want 0", got)
	}
}

func TestActualCostUSDNilEngineIsZero(t *testing.T) {
	cfg := &ChatOptions{}
	if got := cfg.actualCostUSD("cheap-general", "openai", 100, 100); got != 0 {
		t.Errorf("nil engine cost = %v, want 0", got)
	}
}
