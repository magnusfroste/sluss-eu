package policy

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/registry"
)

// ROUTER_POLICY_PATH=builtin:pii-local must resolve from the embedded policy set
// with no file on disk — the guarantee that makes it work in the binary-only
// container image.
func TestNewRuntimeCacheLoadsBuiltinPolicy(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := NewRuntimeCache(snap, "builtin:pii-local"); err != nil {
		t.Fatalf("builtin:pii-local should load, got %v", err)
	}
	// Whitespace around the value is tolerated by the caller (TrimSpace), but the
	// embedded name itself must match exactly.
	_, err = NewRuntimeCache(snap, "builtin:does-not-exist")
	if err == nil {
		t.Fatal("unknown built-in name should error")
	}
	if !strings.Contains(err.Error(), "pii-local") {
		t.Fatalf("error should list available built-ins, got %v", err)
	}
}

func TestBuiltinNamesIncludesPiiLocal(t *testing.T) {
	var found bool
	for _, n := range builtin.Names() {
		if n == "pii-local" {
			found = true
		}
	}
	if !found {
		t.Fatalf("built-in names should include pii-local, got %v", builtin.Names())
	}
}

// The shipped NIS2 baseline loads as a built-in policy (ROUTER_POLICY_PATH=
// builtin:nis2-baseline) — the CISO starter ruleset.
func TestNewRuntimeCacheLoadsNIS2Baseline(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := NewRuntimeCache(snap, "builtin:nis2-baseline"); err != nil {
		t.Fatalf("builtin:nis2-baseline should load, got %v", err)
	}
	var found bool
	for _, n := range builtin.Names() {
		if n == "nis2-baseline" {
			found = true
		}
	}
	if !found {
		t.Fatalf("built-in names should include nis2-baseline, got %v", builtin.Names())
	}
}
