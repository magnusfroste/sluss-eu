# ISSUE-008: Skapa model registry schema

## Labels
- `epic: EPIC-02`
- `priority: P0`
- `type: data`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Skapa Postgres-schema för model registry så routern kan ladda providers, models och registry-versioner deterministiskt vid startup, med metadata för capabilities, kostnad, kvalitet och provider-specifika modell-id:n.

## Implementation Contract

- Lägg till databasmigrationer för registry-tabeller som minst täcker:
  - `providers`
  - `models`
  - registry versioning, till exempel `model_registry_versions` eller etablerad repo-konvention.
- `providers` ska minst stödja `id`, `name`, `status`, `base_url` och `auth_secret_ref`.
- `models` ska minst stödja `id`, `provider_id`, `provider_model_id`, `tier`, JSONB metadatafält och `enabled`.
- Modellrader ska ha provider-specifika modell-id:n via `provider_model_id`; policy och routing ska kunna använda interna modell-id:n utan att binda sig till marknadsföringsnamn.
- JSONB metadata ska minst täcka:
  - capabilities, till exempel chat, streaming, tool calls, JSON schema, vision och long context.
  - cost, till exempel input/output-pris per miljon tokens och relevant valuta/enhet.
  - quality metadata, till exempel score per task class eller routingkategori.
  - latency metadata om repoets registrykontrakt förväntar det.
- `providers.status` och `models.enabled` ska göra det möjligt att filtrera bort inaktiva providers/modeller vid startup load.
- Registry versioning ska kunna markera vilken version som är aktiv och vilken version ett routingbeslut senare ska kunna logga.
- Startup load ska kunna hämta aktiv registry-version, aktiva providers och enabled models utan full table scans på normal datamängd.
- Föreslagna index:
  - aktiv registry-version, till exempel partial index på active/current status.
  - `providers.status`.
  - `models.provider_id`.
  - `models.enabled`.
  - unik constraint/index för `models(provider_id, provider_model_id)`.
  - lookup på intern `models.id`.
- Schema ska spegla designen i `01-architecture/06-model-registry.md` och datamodellen i `01-architecture/09-data-model.md`.

## Files / Packages

- Förväntade migrationsfiler under repoets befintliga databas-/migrationstruktur.
- Förväntad seed/dev fixture för minst en provider, en eller flera modeller och en aktiv registry-version om repoets lokala startup behöver registrydata.
- Förväntade SQL- eller repositorytester nära databaskoden om sådan teststruktur finns.
- Skapa ingen provideradapter, routerlogik eller admin endpoint i detta issue.

## Acceptance Criteria

- En tom lokal Postgres-databas kan migreras och innehåller registry-tabeller för providers, models och versioning.
- Providers kan lagra status, base URL och secret reference utan att lagra faktiska provider-secrets i klartext.
- Models kan lagra intern modellidentitet, provider relation, `provider_model_id`, tier, enabled flag och JSONB metadata för capabilities, cost och quality.
- Registry versioning kan ange en aktiv/current version som startup load kan välja deterministiskt.
- Index/constraints stödjer snabb startup load av aktiv registrydata.
- Seed eller testfixture, om tillagd, skapar en minimal aktiv registry som följer `01-architecture/06-model-registry.md`.
- Schema möjliggör att routingbeslut senare kan logga `model_registry_version`.

## Tests / Verification

- Kör migrationerna mot en tom lokal Postgres-databas.
- Verifiera med fokuserad SQL eller repository-test att providers, models och versioning-tabeller finns.
- Verifiera constraints/index för provider/model relationer, enabled/status-filter och provider model ID-unikhet.
- Verifiera att JSONB metadata kan insertas och läsas för capabilities, cost och quality.
- Verifiera att aktiv registry-version plus enabled models kan hämtas med den query-form som startup load förväntas använda.
- Kör relevant fokuserad Go-test eller migrations-testkommando. Om repoets breda gate är snabb nog, kör även `make test` eller motsvarande.

## Out of Scope

- Admin API för registry reload.
- Dynamic provider health job eller runtime health schema.
- Automatisk prisuppdatering från providers.
- Routerbeslut, fallbackkedjor och policyregler.
- Provideradapter-implementationer eller faktiska externa provideranrop.

## Dependencies

- Registrydesign: `01-architecture/06-model-registry.md`.
- Datamodell: `01-architecture/09-data-model.md`.
- Senare routing/logging-arbete som ska skriva `model_registry_version` på beslut.

## Subagent Notes

- Implementerande agent ska skapa migrationer, seed vid behov och verifiering, inte bygga runtime-routerlogik.
- Håll fast path startup-vänlig: registry ska kunna laddas en gång till minne och därefter användas utan databasrundtur i routingens heta väg.
- Separera statisk registrydata från dynamisk provider health enligt architecture-dokumentet.

## Klar när

- Registry schema, seed vid behov och verifiering är implementerade enligt acceptance criteria.
- Startup load har ett tydligt aktivt registry att läsa från utan per-request DB-access.
- Fokuserade migrations-/SQL-/repositorytester passerar.

## Closeout 2026-05-19

- Implementerat registryschema i `db/migrations/001_foundation.sql` med `model_registry_versions`, `providers` och `models`.
- Seed skapar en aktuell aktiv registryversion, två aktiva providers och enabled models med JSONB metadata för capabilities, cost, quality och latency.
- Verifierat mot isolerad Postgres på port `55432`; invariants kontrollerade för current registry, enabled active models, index och hash-only API key.
