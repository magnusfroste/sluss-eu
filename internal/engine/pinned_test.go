package engine_test

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestDecidePinnedUnknownModelReturnsModelNotFound(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	eng := engine.New(store)

	bogus := "nonexistent-model-xyz"
	job := &router.JobDescriptor{
		RequestID:     "req_pin",
		TaskType:      router.TaskSimpleChat,
		RiskLevel:     router.RiskLow,
		RouterMode:    router.RouterModeAuto,
		ExplicitModel: &bogus,
	}
	_, err = eng.Decide(job, nil, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrModelNotFound) {
		t.Fatalf("pinned unknown model error = %v, want ErrModelNotFound", err)
	}
	// A valid registry model must still route fine.
	good := "cheap-general"
	job.ExplicitModel = &good
	if _, err := eng.Decide(job, nil, engine.FullyHealthy, false); err != nil {
		t.Fatalf("valid pinned model should route, got %v", err)
	}
}
