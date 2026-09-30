package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
)

// ISSUE-116: the policy console lists the active rules in plain words, with
// the dry-run before the rule editor and the destructive actions last.
func TestPolicyPageShowsActiveRulesAndOrder(t *testing.T) {
	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewRuntimeCache(snap, "builtin:nis2-baseline")
	if err != nil {
		t.Fatal(err)
	}
	h := PolicyPageHandler(PolicyPageOptions{Cache: cache})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/policy", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "data is pii") || !strings.Contains(body, "only models tagged local") {
		t.Fatal("active rules not rendered in plain words")
	}
	dry := strings.Index(body, "Dry-run — test a prompt")
	editor := strings.Index(body, "Rules — no YAML")
	danger := strings.Index(body, "danger zone")
	if !(dry > 0 && dry < editor && editor < danger) {
		t.Fatalf("section order wrong: dry=%d editor=%d danger=%d", dry, editor, danger)
	}
}
