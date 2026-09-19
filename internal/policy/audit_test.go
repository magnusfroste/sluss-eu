package policy

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
)

func TestReloadAuditsSuccessAndFailure(t *testing.T) {
	snapshot := testSnapshot(t)
	mem := audit.NewMemorySink(0)

	cache, err := NewCache([]Source{{Policy: mustParse(t, validPolicy), Registry: snapshot}})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	cache.SetAuditor(mem)

	// Successful reload → one success entry carrying the new version.
	next := mustParse(t, strings.Replace(validPolicy, "pv_2026_05_19", "pv_next", 1))
	if err := cache.Reload([]Source{{Policy: next, Registry: snapshot}}); err != nil {
		t.Fatalf("valid reload: %v", err)
	}

	// Rejected reload → one failure entry.
	bad := mustParse(t, strings.Replace(validPolicy, "premium-reasoning", "ghost-model", 1))
	if err := cache.Reload([]Source{{Policy: bad, Registry: snapshot}}); err == nil {
		t.Fatalf("expected reload validation error")
	}

	entries := mem.Entries()
	if len(entries) != 2 {
		t.Fatalf("want 2 audit entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionPolicyReload || entries[0].Outcome != audit.OutcomeSuccess {
		t.Errorf("first entry should be a successful reload: %+v", entries[0])
	}
	if entries[0].Target != "pv_next" {
		t.Errorf("success entry target = %q, want pv_next", entries[0].Target)
	}
	if entries[1].Outcome != audit.OutcomeFailure {
		t.Errorf("second entry should be a failure: %+v", entries[1])
	}
	if entries[1].Reason == "" {
		t.Error("failure entry should carry a reason")
	}
}
