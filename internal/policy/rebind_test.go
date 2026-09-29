package policy

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

// ISSUE-115: a live roster swap re-validates the active policy against the new
// registry. Dropping a model the policy forces must be rejected with the old
// policy still active; a compatible roster rebinds (new registry version).
func TestCacheRebindTwoPhase(t *testing.T) {
	cache, err := NewCache([]Source{{Policy: mustParse(t, validPolicy), Registry: testSnapshot(t)}})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	// Roster without the forced model → rejected, nothing changes.
	def := registry.DefaultDefinition()
	kept := def.Models[:0]
	for _, m := range def.Models {
		if m.ID != "premium-reasoning" {
			kept = append(kept, m)
		}
	}
	def.Models = kept
	def.RegistryVersion = "reg-without-premium"
	without, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := cache.Rebind(without); err == nil {
		t.Fatalf("expected rebind to fail when a forced model is dropped")
	}
	if p, _ := cache.Active(Scope{}); p.RegistryVersion() != registry.DefaultDefinition().RegistryVersion {
		t.Fatalf("active policy changed after rejected rebind: %s", p.RegistryVersion())
	}

	// Compatible roster → prepared, but not live until commit.
	def2 := registry.DefaultDefinition()
	def2.RegistryVersion = "reg-next"
	next, err := registry.NewSnapshot(def2)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	commit, err := cache.Rebind(next)
	if err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	if p, _ := cache.Active(Scope{}); p.RegistryVersion() == "reg-next" {
		t.Fatalf("rebind must not apply before commit")
	}
	commit()
	p, _ := cache.Active(Scope{})
	if p.RegistryVersion() != "reg-next" || p.Version() != "pv_2026_05_19" {
		t.Fatalf("after commit: registry=%s policy=%s", p.RegistryVersion(), p.Version())
	}

	// Nil / empty caches are harmless no-ops.
	var nilCache *Cache
	if c, err := nilCache.Rebind(next); err != nil || c == nil {
		t.Fatalf("nil cache rebind: %v", err)
	}
}
