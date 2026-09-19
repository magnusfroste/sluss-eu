package providercfg

import (
	"path/filepath"
	"testing"
)

func sampleProvider() Provider {
	return Provider{ID: "zai", Name: "Z.ai", BaseURL: "https://api.z.ai/api/coding/paas/v4", KeyEnv: "ZAI_API_KEY"}
}

func sampleModel() Model {
	return Model{ID: "zai-glm", ProviderID: "zai", ProviderModelID: "glm-4.6", Tier: "balanced", InputUSDPerMTok: 0.6, OutputUSDPerMTok: 2.2, Enabled: true}
}

func TestProviderSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	if got, err := Load(path); err != nil || got != nil {
		t.Fatalf("missing file should be empty/no-error: %v %v", got, err)
	}
	if err := Save(path, []Provider{sampleProvider()}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil || len(got) != 1 || got[0].ID != "zai" || got[0].KeyEnv != "ZAI_API_KEY" {
		t.Fatalf("round-trip failed: %+v err=%v", got, err)
	}
}

func TestModelSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	if got, err := LoadModels(path); err != nil || got != nil {
		t.Fatalf("missing file should be empty/no-error: %v %v", got, err)
	}
	if err := SaveModels(path, []Model{sampleModel()}); err != nil {
		t.Fatalf("SaveModels: %v", err)
	}
	got, err := LoadModels(path)
	if err != nil || len(got) != 1 || got[0].ID != "zai-glm" || got[0].ProviderID != "zai" || got[0].ProviderModelID != "glm-4.6" {
		t.Fatalf("round-trip failed: %+v err=%v", got, err)
	}
}

func TestUpsertAndDelete(t *testing.T) {
	ps := []Provider{sampleProvider()}
	ps = Upsert(ps, Provider{ID: "zai", Name: "Z.ai v2", BaseURL: "https://x", KeyEnv: "K"})
	if len(ps) != 1 || ps[0].Name != "Z.ai v2" {
		t.Fatalf("upsert should replace by id: %+v", ps)
	}
	ps = Upsert(ps, Provider{ID: "other"})
	if len(ps) != 2 {
		t.Fatalf("upsert should add new: %+v", ps)
	}
	ps = Delete(ps, "zai")
	if len(ps) != 1 || ps[0].ID != "other" {
		t.Fatalf("delete failed: %+v", ps)
	}
}

func TestUpsertAndDeleteModel(t *testing.T) {
	ms := []Model{sampleModel()}
	ms = UpsertModel(ms, Model{ID: "zai-glm", ProviderID: "zai", ProviderModelID: "glm-4.6", Tier: "premium"})
	if len(ms) != 1 || ms[0].Tier != "premium" {
		t.Fatalf("upsert should replace by id and re-tier: %+v", ms)
	}
	ms = UpsertModel(ms, Model{ID: "other", ProviderID: "zai", ProviderModelID: "m", Tier: "cheap"})
	if len(ms) != 2 {
		t.Fatalf("upsert should add new: %+v", ms)
	}
	ms = DeleteModel(ms, "zai-glm")
	if len(ms) != 1 || ms[0].ID != "other" {
		t.Fatalf("delete failed: %+v", ms)
	}
}

func TestProviderValidate(t *testing.T) {
	p := sampleProvider()
	if err := p.Validate(); err != nil {
		t.Fatalf("valid provider rejected: %v", err)
	}
	bad := sampleProvider()
	bad.BaseURL = "ftp://x"
	if err := bad.Validate(); err == nil {
		t.Error("non-http base_url should be rejected")
	}
	bad2 := sampleProvider()
	bad2.KeyEnv = ""
	if err := bad2.Validate(); err == nil {
		t.Error("missing key_env should be rejected")
	}
	p2 := sampleProvider()
	p2.ID = "Z.AI Coding!"
	_ = p2.Validate()
	if p2.ID != "z-ai-coding" {
		t.Errorf("id not normalized: %q", p2.ID)
	}
}

func TestModelValidate(t *testing.T) {
	m := sampleModel()
	if err := m.Validate(); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}
	bad := sampleModel()
	bad.Tier = "ultra"
	if err := bad.Validate(); err == nil {
		t.Error("invalid tier should be rejected")
	}
	bad2 := sampleModel()
	bad2.ProviderID = ""
	if err := bad2.Validate(); err == nil {
		t.Error("missing provider should be rejected")
	}
	bad3 := sampleModel()
	bad3.ProviderModelID = ""
	if err := bad3.Validate(); err == nil {
		t.Error("missing provider_model_id should be rejected")
	}
}

func TestRegistryEntriesKeyGating(t *testing.T) {
	provs, models := RegistryEntries([]Provider{sampleProvider()}, []Model{sampleModel()})
	if len(provs) != 1 || provs[0].AuthSecretRef != "ZAI_API_KEY" || provs[0].BaseURL == "" {
		t.Fatalf("provider entry wrong: %+v", provs)
	}
	if len(models) != 1 || models[0].ProviderID != "zai" || models[0].Enabled {
		t.Fatalf("model should be disabled without key: %+v", models)
	}
	if models[0].Cost.InputMicrosPerMillionToken != 600000 {
		t.Errorf("pricing conversion wrong: %d", models[0].Cost.InputMicrosPerMillionToken)
	}

	t.Setenv("ZAI_API_KEY", "secret")
	_, models = RegistryEntries([]Provider{sampleProvider()}, []Model{sampleModel()})
	if !models[0].Enabled {
		t.Error("model should be enabled once key is present")
	}
}

// ReasoningCapable flows from the roster into the registry model so the router
// can control enable_thinking per request.
func TestRegistryEntriesCarryReasoningCapable(t *testing.T) {
	m := sampleModel()
	m.ReasoningCapable = true
	_, models := RegistryEntries([]Provider{sampleProvider()}, []Model{m})
	if len(models) != 1 || !models[0].ReasoningCapable {
		t.Fatalf("reasoning_capable lost in registry mapping: %+v", models)
	}
	m.ReasoningCapable = false
	_, models = RegistryEntries([]Provider{sampleProvider()}, []Model{m})
	if models[0].ReasoningCapable {
		t.Fatal("reasoning_capable should default off")
	}
}

func TestRegistryEntriesBuiltinMetadataPreserved(t *testing.T) {
	// A re-tiered built-in keeps its rich seed metadata (capabilities/context)
	// even when tier and price are changed on the Models page.
	m := Model{ID: "premium-reasoning", ProviderID: "openrouter", ProviderModelID: "anthropic/claude-sonnet-4.5", Tier: "cheap", Enabled: true}
	_, models := RegistryEntries(SeedProviders(), []Model{m})
	if len(models) != 1 {
		t.Fatalf("want 1 model, got %d", len(models))
	}
	if models[0].Tier != "cheap" {
		t.Errorf("re-tier not applied: %s", models[0].Tier)
	}
	if !models[0].Capabilities.LongContext || models[0].ContextWindowTokens != 1000000 {
		t.Errorf("built-in rich metadata lost: %+v", models[0].Capabilities)
	}
}
