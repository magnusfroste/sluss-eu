package providercfg

// Roster-in-SQLite (ISSUE-073): the roster (provider connections + routable
// models) moves from providers.json / models.json into the durable SQLite store
// (history.db) so there is ONE source of truth for which model fills which tier.
// providercfg keeps owning the roster *shape* (types, validation, seed, registry
// projection); RosterStore abstracts the persistence so this package never
// imports internal/history (which imports it — that would be a cycle).

import (
	"fmt"
	"os"
)

// RosterStore is the persistence backend for the roster. *history.Store
// satisfies it structurally, so the SQLite file becomes the roster's home
// without providercfg depending on the history package.
type RosterStore interface {
	LoadRosterProviders() ([]Provider, error)
	LoadRosterModels() ([]Model, error)
	UpsertRosterProvider(Provider) error
	UpsertRosterModel(Model) error
	DeleteRosterProvider(id string) error
	DeleteRosterModel(id string) error
}

// MigrateAndSeedDB makes the DB roster authoritative on startup. It is a no-op
// when the DB roster is already populated, so admin edits (including deletions)
// are never clobbered. On an empty DB it either:
//
//   - imports an existing models.json / providers.json (from ISSUE-072) into the
//     tables and renames the files ".imported" (mirroring the spend.json → history
//     migration), or
//   - seeds the built-in OpenRouter connection + three tier models when no such
//     files exist (a fresh install).
//
// migrated reports a JSON import happened; seeded reports the built-ins were
// planted. Prices in the files are USD/Mtok floats; the store converts them to
// its integer micro-USD columns.
func MigrateAndSeedDB(store RosterStore, providersPath, modelsPath string) (migrated, seeded bool, err error) {
	if store == nil {
		return false, false, nil
	}
	provs, err := store.LoadRosterProviders()
	if err != nil {
		return false, false, fmt.Errorf("providercfg: read db providers: %w", err)
	}
	models, err := store.LoadRosterModels()
	if err != nil {
		return false, false, fmt.Errorf("providercfg: read db models: %w", err)
	}
	if len(provs) > 0 || len(models) > 0 {
		return false, false, nil // already set up; respect admin edits
	}

	// Import a prior file-based roster if present (ISSUE-072 left split files).
	fileProvs, ferr := Load(providersPath)
	if ferr != nil {
		return false, false, ferr
	}
	fileModels, ferr := LoadModels(modelsPath)
	if ferr != nil {
		return false, false, ferr
	}
	if len(fileProvs) > 0 || len(fileModels) > 0 {
		for _, p := range fileProvs {
			if err := store.UpsertRosterProvider(p); err != nil {
				return false, false, err
			}
		}
		for _, m := range fileModels {
			if err := store.UpsertRosterModel(m); err != nil {
				return false, false, err
			}
		}
		// Rename so the files aren't re-read or mistaken for the source of truth.
		renameImported(modelsPath)
		renameImported(providersPath)
		return true, false, nil
	}

	// Fresh install: plant the built-ins as data (nothing hardcoded).
	for _, p := range SeedProviders() {
		if err := store.UpsertRosterProvider(p); err != nil {
			return false, false, err
		}
	}
	for _, m := range SeedModels() {
		if err := store.UpsertRosterModel(m); err != nil {
			return false, true, err
		}
	}
	return false, true, nil
}

// renameImported renames path to path+".imported" (best effort; a missing file
// or rename error is ignored — the import already succeeded).
func renameImported(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Rename(path, path+".imported")
	}
}

// ApplyTierOverridesToDB resolves the legacy ROUTER_MODEL_<TIER> env overrides
// against the DB roster: for every enabled tier with a non-empty override it
// rewrites the ProviderModelID of that tier's models to the override slug and
// persists the change. This keeps the DB (now the source of truth) and routing
// in agreement — without it the override would change routing but not the
// persisted roster shown on the Models page. Returns the number of models
// updated. overrides is keyed by tier ("cheap" / "balanced" / "premium").
func ApplyTierOverridesToDB(store RosterStore, overrides map[string]string) (int, error) {
	if store == nil || len(overrides) == 0 {
		return 0, nil
	}
	models, err := store.LoadRosterModels()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range models {
		slug := overrides[m.Tier]
		if slug == "" || slug == m.ProviderModelID {
			continue
		}
		m.ProviderModelID = slug
		if err := store.UpsertRosterModel(m); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
