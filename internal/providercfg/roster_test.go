package providercfg

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRoster is an in-memory RosterStore for exercising the DB migration/seed
// helpers without importing internal/history (which would be an import cycle).
type fakeRoster struct {
	provs  map[string]Provider
	models map[string]Model
}

func newFakeRoster() *fakeRoster {
	return &fakeRoster{provs: map[string]Provider{}, models: map[string]Model{}}
}

func (f *fakeRoster) LoadRosterProviders() ([]Provider, error) {
	out := make([]Provider, 0, len(f.provs))
	for _, p := range f.provs {
		out = append(out, p)
	}
	return SortByID(out), nil
}

func (f *fakeRoster) LoadRosterModels() ([]Model, error) {
	out := make([]Model, 0, len(f.models))
	for _, m := range f.models {
		out = append(out, m)
	}
	return SortModels(out), nil
}

func (f *fakeRoster) UpsertRosterProvider(p Provider) error { f.provs[p.ID] = p; return nil }
func (f *fakeRoster) UpsertRosterModel(m Model) error       { f.models[m.ID] = m; return nil }
func (f *fakeRoster) DeleteRosterProvider(id string) error  { delete(f.provs, id); return nil }
func (f *fakeRoster) DeleteRosterModel(id string) error     { delete(f.models, id); return nil }

func TestMigrateAndSeedDBSeedsOnEmpty(t *testing.T) {
	store := newFakeRoster()
	// No data dir files → seed the built-ins.
	migrated, seeded, err := MigrateAndSeedDB(store, "", "")
	if err != nil || migrated || !seeded {
		t.Fatalf("fresh install: migrated=%v seeded=%v err=%v", migrated, seeded, err)
	}
	provs, _ := store.LoadRosterProviders()
	if len(provs) != 1 || provs[0].ID != "openrouter" {
		t.Fatalf("openrouter connection not seeded: %+v", provs)
	}
	models, _ := store.LoadRosterModels()
	ids := map[string]bool{}
	for _, m := range models {
		ids[m.ID] = true
	}
	for _, want := range []string{"cheap-general", "balanced-coder", "premium-reasoning"} {
		if !ids[want] {
			t.Errorf("built-in %q not seeded", want)
		}
	}
	// Idempotent: a second call is a no-op (DB now populated).
	migrated, seeded, err = MigrateAndSeedDB(store, "", "")
	if err != nil || migrated || seeded {
		t.Fatalf("second call should be a no-op: migrated=%v seeded=%v err=%v", migrated, seeded, err)
	}
}

func TestMigrateAndSeedDBImportsJSONFiles(t *testing.T) {
	dir := t.TempDir()
	pf := filepath.Join(dir, "providers.json")
	mf := filepath.Join(dir, "models.json")
	if err := Save(pf, []Provider{sampleProvider()}); err != nil {
		t.Fatal(err)
	}
	if err := SaveModels(mf, []Model{sampleModel()}); err != nil {
		t.Fatal(err)
	}

	store := newFakeRoster()
	migrated, seeded, err := MigrateAndSeedDB(store, pf, mf)
	if err != nil || !migrated || seeded {
		t.Fatalf("expected import: migrated=%v seeded=%v err=%v", migrated, seeded, err)
	}
	// Roster came from the files (no built-in seed layered on top).
	provs, _ := store.LoadRosterProviders()
	if len(provs) != 1 || provs[0].ID != "zai" {
		t.Fatalf("imported provider wrong: %+v", provs)
	}
	models, _ := store.LoadRosterModels()
	if len(models) != 1 || models[0].ID != "zai-glm" || models[0].InputUSDPerMTok != 0.6 {
		t.Fatalf("imported model wrong: %+v", models)
	}
	// Files renamed so they aren't re-imported / mistaken for the source.
	if _, err := os.Stat(mf); !os.IsNotExist(err) {
		t.Errorf("models.json should be renamed after import")
	}
	if _, err := os.Stat(mf + ".imported"); err != nil {
		t.Errorf("models.json.imported should exist: %v", err)
	}
	if _, err := os.Stat(pf + ".imported"); err != nil {
		t.Errorf("providers.json.imported should exist: %v", err)
	}
}

func TestMigrateAndSeedDBRespectsExisting(t *testing.T) {
	store := newFakeRoster()
	store.provs["custom"] = Provider{ID: "custom", BaseURL: "https://x", KeyEnv: "K"}
	// Even with JSON files present, a non-empty DB is never clobbered.
	dir := t.TempDir()
	mf := filepath.Join(dir, "models.json")
	_ = SaveModels(mf, []Model{sampleModel()})
	migrated, seeded, err := MigrateAndSeedDB(store, "", mf)
	if err != nil || migrated || seeded {
		t.Fatalf("populated DB: migrated=%v seeded=%v err=%v", migrated, seeded, err)
	}
	if _, err := os.Stat(mf); err != nil {
		t.Errorf("models.json must not be touched when DB is populated: %v", err)
	}
	provs, _ := store.LoadRosterProviders()
	if len(provs) != 1 || provs[0].ID != "custom" {
		t.Errorf("existing roster changed: %+v", provs)
	}
}

func TestApplyTierOverridesToDB(t *testing.T) {
	store := newFakeRoster()
	if _, _, err := MigrateAndSeedDB(store, "", ""); err != nil {
		t.Fatal(err)
	}
	// Override the premium tier's slug; write through to the DB.
	n, err := ApplyTierOverridesToDB(store, map[string]string{"premium": "z-ai/glm-4.6"})
	if err != nil || n != 1 {
		t.Fatalf("override write-through: n=%d err=%v", n, err)
	}
	models, _ := store.LoadRosterModels()
	for _, m := range models {
		if m.Tier == "premium" && m.ProviderModelID != "z-ai/glm-4.6" {
			t.Errorf("premium slug not written through: %+v", m)
		}
		if m.Tier != "premium" && m.ProviderModelID == "z-ai/glm-4.6" {
			t.Errorf("non-premium model wrongly rewritten: %+v", m)
		}
	}
	// Idempotent: re-applying the same override updates nothing.
	if n, _ := ApplyTierOverridesToDB(store, map[string]string{"premium": "z-ai/glm-4.6"}); n != 0 {
		t.Errorf("re-apply should be a no-op, updated %d", n)
	}
	// Empty override map does nothing.
	if n, _ := ApplyTierOverridesToDB(store, nil); n != 0 {
		t.Errorf("nil overrides should update nothing, got %d", n)
	}
}
