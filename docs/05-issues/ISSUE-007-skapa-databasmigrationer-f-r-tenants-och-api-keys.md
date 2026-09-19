# ISSUE-007: Skapa databasmigrationer för tenants och api_keys

## Labels
- `epic: EPIC-09`
- `priority: P0`
- `type: data`
- `sprint: 01`
- `category: enhancement`
- `state: done`

## Intent

Skapa Postgres identity foundation för router-gatewayn: tenants, projects och api_keys ska kunna lagras, seedas lokalt och användas av auth/policy-lager utan att plaintext API keys någonsin persisteras.

## Implementation Contract

- Lägg till databasmigrationer för `tenants`, `projects` och `api_keys` enligt `01-architecture/09-data-model.md`.
- `tenants` ska minst stödja `id`, `name`, `status`, `retention_days` och `created_at`.
- `projects` ska minst stödja `id`, `tenant_id`, `name`, `default_policy_version_id` och `created_at`.
- `api_keys` ska minst stödja `id`, `tenant_id`, `project_id`, `key_hash`, `name`, `scopes`, `status`, `created_at` och `last_used_at`.
- Relationer ska vara explicita:
  - `projects.tenant_id` refererar `tenants.id`.
  - `api_keys.tenant_id` refererar `tenants.id`.
  - `api_keys.project_id` refererar `projects.id`.
- API keys får endast lagras hashade i `api_keys.key_hash`.
- Migrationer, seed scripts och test fixtures får inte innehålla eller lagra plaintext API keys i databasen.
- Lokal utveckling ska få en seedad tenant, project och API key-identitet där endast hashad key sparas. Plaintext-värdet får bara exponeras som lokal fixture/env-instruktion om det behövs för test, inte som persisterad rad.
- Statusfält ska vara deterministiska och lämpade för auth-filtering, till exempel `active`, `disabled` eller motsvarande etablerad repo-konvention.
- Index ska finnas för auth/startup-relevanta accessmönster:
  - unik lookup på `api_keys.key_hash`.
  - lookup på `projects.tenant_id`.
  - lookup på `api_keys.tenant_id`.
  - lookup på `api_keys.project_id`.
- Migrationerna ska vara idempotenta via repoets vanliga migration runner och ska kunna köras från tom lokal databas.

## Files / Packages

- Förväntade migrationsfiler under repoets befintliga databas-/migrationstruktur.
- Förväntad seed/dev fixture i repoets befintliga seed- eller testdataflöde.
- Förväntade repository- eller SQL-verifieringstester nära databaskoden om sådan teststruktur finns.
- Skapa ingen produktkod utöver migrations-/seed-/testkontrakt som behövs för databasschemat.

## Acceptance Criteria

- En tom lokal Postgres-databas kan migreras och innehåller tabellerna `tenants`, `projects` och `api_keys`.
- Foreign keys skyddar tenant -> project -> api_key-relationerna.
- `api_keys.key_hash` är obligatoriskt, indexerat och kan användas för snabb key lookup.
- Inga kolumner, seeds eller fixtures persisterar plaintext API keys.
- Lokal seed skapar minst en aktiv tenant, ett aktivt/användbart project och en aktiv API key-rad med hashad key.
- Seedade rader har stabila identifierare eller stabil lookup så integrationstester kan använda dem deterministiskt.
- Schema och seed följer datamodellen i `01-architecture/09-data-model.md`.

## Tests / Verification

- Kör migrationerna mot en tom lokal Postgres-databas.
- Verifiera med fokuserad SQL eller repository-test att tabeller, constraints och index finns.
- Verifiera med fokuserad SQL eller repository-test att seedad API key endast finns som hash i `api_keys.key_hash`.
- Verifiera att `api_keys.key_hash` kan användas för lookup utan full table scan när index-inspektion är praktiskt i testmiljön.
- Kör relevant fokuserad Go-test eller migrations-testkommando. Om repoets breda gate är snabb nog, kör även `make test` eller motsvarande.

## Out of Scope

- Runtime auth middleware.
- API key issuance, rotation, revocation UI eller admin endpoints.
- Tenant billing, quotas eller policy authoring.
- Request logging, outcomes och route attempts-tabeller.
- Produktionshemlighetshantering utöver att plaintext inte persisteras.

## Dependencies

- Datamodell: `01-architecture/09-data-model.md`.
- Repoets befintliga migration runner och lokala Postgres-konfiguration.
- Kommande auth/policy-arbete som konsumerar tenant, project och key identity.

## Subagent Notes

- Implementerande agent ska skapa migrationer, seed och verifiering enligt kontraktet ovan.
- Bevara fast path-principen: identity lookup får stödja snabb auth, men routingens fast path ska inte bero på externa tjänster eller LLM-anrop.
- Var särskilt hård mot oavsiktlig plaintext-persistens i seeds och testfixtures.

## Klar när

- Migrationer, seed och verifiering är implementerade enligt acceptance criteria.
- Plaintext API keys persisteras inte i schema, seed eller testfixtures.
- Fokuserade migrations-/SQL-/repositorytester passerar.

## Closeout 2026-05-19

- Implementerat i `db/migrations/001_foundation.sql`, `db/seeds/local.sql`, `db/README.md`, `.env.example` och `Makefile`.
- `api_keys.key_hash` kräver `sha256:<64 hex>` och seed sparar endast hash för `local_router_key`.
- Verifierat mot isolerad Postgres på port `55432` eftersom lokal `5432` var upptagen av `anbud-postgres-1`; `make migrate` och `make seed` kördes två gånger för idempotens.
