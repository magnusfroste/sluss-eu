package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// localRegistryStore builds a registry with a local-tagged provider and a
// cloud provider, for the visible cloud→local switch (ISSUE-083).
func localRegistryStore(t *testing.T) *engine.Engine {
	t.Helper()
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	def := registry.Definition{
		RegistryVersion: "reg-083-test",
		CreatedAt:       time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "cloud", Name: "Cloud", Status: registry.ProviderStatusActive},
			{ID: "onprem", Name: "On-Prem", Status: registry.ProviderStatusActive, ComplianceTags: []string{"local", "eu-resident"}},
			{ID: "privai", Name: "Private DGX", Status: registry.ProviderStatusActive, ComplianceTags: []string{"private", "eu-resident"}},
		},
		Models: []registry.Model{
			{ID: "cloud-model", ProviderID: "cloud", ProviderModelID: "c", Tier: registry.TierBalanced, Capabilities: caps, Cost: cost, Enabled: true},
			{ID: "local-model", ProviderID: "onprem", ProviderModelID: "l", Tier: registry.TierBalanced, Capabilities: caps, Cost: cost, Enabled: true},
			{ID: "private-model", ProviderID: "privai", ProviderModelID: "p", Tier: registry.TierBalanced, Capabilities: caps, Cost: cost, Enabled: true},
		},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return engine.New(store)
}

func TestRouteReasonHeadersLocalEgressWithPII(t *testing.T) {
	cfg := &ChatOptions{Engine: localRegistryStore(t)}
	job := &router.JobDescriptor{TaskType: router.TaskSimpleChat, Sensitivity: router.SensitivityPII, PIITypes: []string{"personnummer"}}
	rec := httptest.NewRecorder()
	cfg.setRouteReasonHeaders(rec, job, engine.RouteDecision{SelectedModel: "local-model"})

	// Header value must be ASCII (HTTP headers aren't UTF-8 safe): a machine code
	// "pii:<types>", not a localized sentence.
	got := rec.Header().Get("X-Router-Route-Reason")
	if got != "pii:personnummer" {
		t.Fatalf("reason = %q, want pii:personnummer", got)
	}
	if !isASCII(got) {
		t.Fatalf("route-reason header must be ASCII, got %q", got)
	}
	if got := rec.Header().Get("X-Router-Egress"); got != "local" {
		t.Fatalf("egress = %q, want local", got)
	}
	if got := rec.Header().Get("X-Router-Selected-Tags"); got == "" {
		t.Fatal("selected tags header missing")
	}
	// Never leak a PII value.
	if v := rec.Header().Get("X-Router-Route-Reason"); contains083(v, "9876") {
		t.Fatal("PII value leaked into header")
	}
}

func TestRouteReasonHeadersCloudNoPII(t *testing.T) {
	cfg := &ChatOptions{Engine: localRegistryStore(t)}
	job := &router.JobDescriptor{TaskType: router.TaskSimpleChat, Sensitivity: router.SensitivityNone}
	rec := httptest.NewRecorder()
	cfg.setRouteReasonHeaders(rec, job, engine.RouteDecision{SelectedModel: "cloud-model"})

	if got := rec.Header().Get("X-Router-Route-Reason"); got != "" {
		t.Fatalf("reason should be empty for non-sensitive request, got %q", got)
	}
	if got := rec.Header().Get("X-Router-Egress"); got != "cloud" {
		t.Fatalf("egress = %q, want cloud", got)
	}
}

// A "private"-tagged provider (e.g. a self-hosted DGX endpoint) counts as
// staying in the house — egress "local".
func TestRouteReasonPrivateTagIsLocalEgress(t *testing.T) {
	cfg := &ChatOptions{Engine: localRegistryStore(t)}
	job := &router.JobDescriptor{TaskType: router.TaskSimpleChat, Sensitivity: router.SensitivityPII, PIITypes: []string{"personnummer"}}
	rec := httptest.NewRecorder()
	cfg.setRouteReasonHeaders(rec, job, engine.RouteDecision{SelectedModel: "private-model"})
	if got := rec.Header().Get("X-Router-Egress"); got != "local" {
		t.Fatalf("egress for private-tagged model = %q, want local", got)
	}
}

func TestRouteReasonSensitivityWithoutTypes(t *testing.T) {
	cfg := &ChatOptions{Engine: localRegistryStore(t)}
	job := &router.JobDescriptor{TaskType: router.TaskSimpleChat, Sensitivity: router.SensitivityPII}
	rec := httptest.NewRecorder()
	cfg.setRouteReasonHeaders(rec, job, engine.RouteDecision{SelectedModel: "cloud-model"})
	if got := rec.Header().Get("X-Router-Route-Reason"); got != "pii" {
		t.Fatalf("reason = %q, want pii", got)
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func contains083(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
