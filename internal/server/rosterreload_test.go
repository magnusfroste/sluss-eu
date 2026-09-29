package server

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
)

const reloadTestPolicy = `
version: pv_reload_test
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: forced
    when:
      task_type: security_review
    route:
      force:
        model_profile_name: premium-reasoning
`

func reloadDef(version string, dropForced bool) registry.Definition {
	def := registry.DefaultDefinition()
	def.RegistryVersion = version
	if dropForced {
		kept := def.Models[:0]
		for _, m := range def.Models {
			if m.ID != "premium-reasoning" {
				kept = append(kept, m)
			}
		}
		def.Models = kept
	}
	return def
}

// ISSUE-115: a roster edit applies live and atomically, or not at all.
func TestRosterReloaderSwapsAtomicallyOrNotAtAll(t *testing.T) {
	snap0, err := registry.NewSnapshot(reloadDef("v1", false))
	if err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewStore(snap0)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := policy.Parse([]byte(reloadTestPolicy))
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewCache([]policy.Source{{Policy: parsed, Registry: snap0}})
	if err != nil {
		t.Fatal(err)
	}
	sink := audit.NewMemorySink(0)
	a1, a2 := &provider.MockAdapter{BaseURL: "a1"}, &provider.MockAdapter{BaseURL: "a2"}

	var next RosterBuild
	var buildErr error
	rl := NewRosterReloader(map[string]provider.Adapter{"openai": a1})
	rl.Build = func(context.Context) (RosterBuild, error) { return next, buildErr }
	rl.Registry = store
	rl.Caches = []*policy.Cache{cache, nil}
	rl.Auditor = sink

	// Readers race the swaps (run with -race): they must always see a whole map.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if m := rl.Adapters(); m["openai"] == nil {
					t.Error("reader saw an adapter set without openai")
					return
				}
			}
		}
	}()

	// 1) Valid roster → everything swaps; anthropic has no adapter → reported.
	next = RosterBuild{Definition: reloadDef("v2", false), Adapters: map[string]provider.Adapter{"openai": a2}}
	res, err := rl.Reload(context.Background(), "admin@test")
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if rl.Adapters()["openai"] != a2 {
		t.Fatalf("adapter set not swapped")
	}
	if s, _ := store.Active(); s.RegistryVersion() != "v2" {
		t.Fatalf("registry not swapped: %s", s.RegistryVersion())
	}
	if p, _ := cache.Active(policy.Scope{}); p.RegistryVersion() != "v2" {
		t.Fatalf("policy not rebound: %s", p.RegistryVersion())
	}
	if len(res.Unroutable) != 1 || res.Unroutable[0] != "anthropic" {
		t.Fatalf("unroutable = %v", res.Unroutable)
	}
	if !strings.Contains(res.Summary(), "anthropic") {
		t.Fatalf("summary should name the keyless provider: %s", res.Summary())
	}

	// 2) Roster that drops a policy-forced model → rejected; nothing changes.
	next = RosterBuild{Definition: reloadDef("v3", true), Adapters: map[string]provider.Adapter{"openai": a1}}
	if _, err := rl.Reload(context.Background(), "admin@test"); err == nil {
		t.Fatalf("expected rejection when a forced model is dropped")
	}
	if rl.Adapters()["openai"] != a2 {
		t.Fatalf("adapters changed on a rejected reload")
	}
	if s, _ := store.Active(); s.RegistryVersion() != "v2" {
		t.Fatalf("registry changed on a rejected reload: %s", s.RegistryVersion())
	}

	// 3) Build error → rejected; nothing changes.
	buildErr = errors.New("db locked")
	if _, err := rl.Reload(context.Background(), "admin@test"); err == nil || !strings.Contains(err.Error(), "db locked") {
		t.Fatalf("expected build error, got %v", err)
	}
	close(stop)
	wg.Wait()

	// Every attempt is evidence: one success, two failures, with the actor.
	var ok, failed int
	for _, e := range sink.Entries() {
		if e.Action != audit.ActionRosterReload {
			continue
		}
		if e.Actor != "admin@test" {
			t.Fatalf("audit actor = %q", e.Actor)
		}
		if e.Outcome == audit.OutcomeFailure {
			failed++
		} else {
			ok++
		}
	}
	if ok != 1 || failed != 2 {
		t.Fatalf("audit roster.reload ok=%d failed=%d, want 1/2", ok, failed)
	}
}

func TestApplyRosterEditMessages(t *testing.T) {
	if ok, e := applyRosterEdit(context.Background(), nil, "a", "Model x saved"); !strings.Contains(ok, "restart") || e != "" {
		t.Fatalf("nil reloader: ok=%q err=%q", ok, e)
	}
	snap, _ := registry.NewSnapshot(reloadDef("v1", false))
	store, _ := registry.NewStore(snap)
	rl := NewRosterReloader(nil)
	rl.Registry = store
	rl.Build = func(context.Context) (RosterBuild, error) {
		return RosterBuild{Definition: reloadDef("v2", false), Adapters: map[string]provider.Adapter{}}, nil
	}
	if ok, e := applyRosterEdit(context.Background(), rl, "a", "Model x saved"); !strings.Contains(ok, "live now") || e != "" {
		t.Fatalf("success: ok=%q err=%q", ok, e)
	}
	rl.Build = func(context.Context) (RosterBuild, error) { return RosterBuild{}, errors.New("boom") }
	if ok, e := applyRosterEdit(context.Background(), rl, "a", "Model x saved"); ok != "" || !strings.Contains(e, "NOT applied") {
		t.Fatalf("failure: ok=%q err=%q", ok, e)
	}
}
