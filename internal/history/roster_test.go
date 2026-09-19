package history

import (
	"reflect"
	"testing"

	"github.com/magnusfroste/sluss/internal/providercfg"
)

func TestRosterProviderRoundTrip(t *testing.T) {
	s, _ := openTest(t)
	if ps, err := s.LoadRosterProviders(); err != nil || len(ps) != 0 {
		t.Fatalf("empty store: %+v err=%v", ps, err)
	}
	p := providercfg.Provider{ID: "zai", Name: "Z.ai", BaseURL: "https://api.z.ai", KeyEnv: "ZAI_API_KEY",
		ComplianceTags: []string{"eu-resident", "dpa-signed"}}
	if err := s.UpsertRosterProvider(p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ps, err := s.LoadRosterProviders()
	if err != nil || len(ps) != 1 || !reflect.DeepEqual(ps[0], p) {
		t.Fatalf("round-trip failed: %+v err=%v", ps, err)
	}
	// Upsert replaces by id.
	p.Name = "Z.ai v2"
	if err := s.UpsertRosterProvider(p); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	ps, _ = s.LoadRosterProviders()
	if len(ps) != 1 || ps[0].Name != "Z.ai v2" {
		t.Fatalf("upsert should replace by id: %+v", ps)
	}
	if err := s.DeleteRosterProvider("zai"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ps, _ := s.LoadRosterProviders(); len(ps) != 0 {
		t.Fatalf("delete failed: %+v", ps)
	}
}

func TestRosterModelRoundTripAndPricePrecision(t *testing.T) {
	s, _ := openTest(t)
	m := providercfg.Model{
		ID: "zai-glm", ProviderID: "zai", ProviderModelID: "glm-4.6", Tier: "balanced",
		InputUSDPerMTok: 0.6, OutputUSDPerMTok: 2.2, Enabled: true,
		ComplianceTags:   []string{"eu-resident"},
		ReasoningCapable: true,
	}
	if err := s.UpsertRosterModel(m); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ms, err := s.LoadRosterModels()
	if err != nil || len(ms) != 1 {
		t.Fatalf("load: %+v err=%v", ms, err)
	}
	got := ms[0]
	if got.ID != "zai-glm" || got.ProviderID != "zai" || got.ProviderModelID != "glm-4.6" ||
		got.Tier != "balanced" || !got.Enabled {
		t.Fatalf("round-trip fields wrong: %+v", got)
	}
	if !reflect.DeepEqual(got.ComplianceTags, []string{"eu-resident"}) {
		t.Fatalf("compliance tags did not round-trip: %+v", got.ComplianceTags)
	}
	if !got.ReasoningCapable {
		t.Fatal("reasoning_capable did not round-trip")
	}
	// USD/Mtok survives the micro-USD integer column round-trip.
	if got.InputUSDPerMTok != 0.6 || got.OutputUSDPerMTok != 2.2 {
		t.Errorf("price precision lost: in=%v out=%v", got.InputUSDPerMTok, got.OutputUSDPerMTok)
	}
	// Re-tier via upsert, then delete.
	m.Tier = "premium"
	m.Enabled = false
	if err := s.UpsertRosterModel(m); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	ms, _ = s.LoadRosterModels()
	if len(ms) != 1 || ms[0].Tier != "premium" || ms[0].Enabled {
		t.Fatalf("re-tier/disable not applied: %+v", ms)
	}
	if err := s.DeleteRosterModel("zai-glm"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ms, _ := s.LoadRosterModels(); len(ms) != 0 {
		t.Fatalf("delete failed: %+v", ms)
	}
}

func TestRosterPersistsAcrossReopen(t *testing.T) {
	_, path := openTest(t)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.UpsertRosterProvider(providercfg.Provider{ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", KeyEnv: "OPENROUTER_API_KEY"})
	_ = s.UpsertRosterModel(providercfg.Model{ID: "cheap-general", ProviderID: "openrouter", ProviderModelID: "openai/gpt-4o-mini", Tier: "cheap", Enabled: true})
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	ps, _ := s2.LoadRosterProviders()
	ms, _ := s2.LoadRosterModels()
	if len(ps) != 1 || len(ms) != 1 || ms[0].ID != "cheap-general" {
		t.Fatalf("roster did not survive reopen: provs=%+v models=%+v", ps, ms)
	}
}
