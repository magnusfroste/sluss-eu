package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestDashboardEgressSectionRendersProvidersAndAuditWindow(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	mem := audit.NewMemorySink(0)
	audit.Record(context.Background(), mem, audit.Entry{
		Action: audit.ActionRequestBlocked,
		Detail: map[string]string{"block_code": "provider_not_allowed", "pii_types": "personnummer"},
	})

	htmlH, dataH := DashboardHandler(DashboardOptions{
		Engine:           engine.New(store),
		AuditMemory:      mem,
		ConservativeMode: true,
	})

	rec := httptest.NewRecorder()
	htmlH(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	body := rec.Body.String()
	for _, want := range []string{"Egress &amp; Compliance", "conservative mode", "provider_not_allowed", "personnummer", "openai"} {
		if !strings.Contains(body, want) {
			t.Fatalf("egress section missing %q", want)
		}
	}

	// JSON payload carries the structured section too.
	jrec := httptest.NewRecorder()
	dataH(jrec, httptest.NewRequest(http.MethodGet, "/router/dashboard/data", nil))
	j := jrec.Body.String()
	if !strings.Contains(j, `"egress_compliance"`) || !strings.Contains(j, `"conservative_mode":true`) {
		t.Fatalf("json missing egress section: %s", j[:min(len(j), 300)])
	}
}

func TestDashboardLearningSectionShowsBanditArms(t *testing.T) {
	b := bandit.New(1.41)
	b.Record(string(router.TaskHardCodeDebugging), "balanced-coder", 1)
	b.Record(string(router.TaskHardCodeDebugging), "balanced-coder", 1)

	htmlH, _ := DashboardHandler(DashboardOptions{Bandit: b})
	rec := httptest.NewRecorder()
	htmlH(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	body := rec.Body.String()
	for _, want := range []string{"Learning &amp; routing", "hard_code_debugging", "balanced-coder"} {
		if !strings.Contains(body, want) {
			t.Fatalf("learning section missing %q", want)
		}
	}
}

func TestDashboardSectionsHonestEmptyStates(t *testing.T) {
	htmlH, _ := DashboardHandler(DashboardOptions{})
	rec := httptest.NewRecorder()
	htmlH(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "The egress view requires registry/audit") {
		t.Fatal("egress empty state missing")
	}
	if !strings.Contains(body, "Bandit routing is off") {
		t.Fatal("learning empty state (bandit off) missing")
	}
}

func TestDashboardBanditEnabledButNoData(t *testing.T) {
	htmlH, _ := DashboardHandler(DashboardOptions{Bandit: bandit.New(1.41)})
	rec := httptest.NewRecorder()
	htmlH(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	if !strings.Contains(rec.Body.String(), "nothing learned yet") {
		t.Fatal("bandit-enabled-no-data empty state missing")
	}
}
