# ISSUE-106 — Docs-sanering efter pivoten: less is more

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P1
- **State:** done (2026-07-19)
- **Beroende:** ISSUE-105 (docs under `docs/`).

## Bakgrund

Produkten pivoterade (DECISION_LOG 2026-07-05→07-18) från generisk
kostnads-/latensrouter för utvecklare till Sluss — data-suverän gateway med
control + evidence för CISO-köparen. Stora delar av dokumentationen beskrev
fortfarande pre-pivot-produkten och Postgres/Redis-arkitekturen som aldrig
blev runtime. Tre parallella granskningar (produktkorpus, arkitektur+ADR,
engineering/ops) verifierade varje dokument mot koden.

## Borttaget (git-historiken bevarar allt)

- `MANIFEST.md` — handunderhållen fillista frusen pre-pivot, ingen konsument.
- `docs/00-product/01–07` (vision, PRD, personas, user-stories, scope,
  success-metrics, roadmap) — beskrev den förkastade produkten: utvecklarköpare,
  kostnads-north-star, "full enterprise compliance" som uttryckligt non-goal.
  Doc 08/09/13 bär den korrekta bilden.
- `docs/08-templates/routing-policy-template.md` + `eval-case-template.md` —
  döda YAML-tier-modellen (cheap/balanced/premium settings).
- `docs/sprint-01-demo.md` — engångsartefakt, ersatt av beta-checklistan och
  CISO-demon.

## Korrigerat mot kodverifierat facit

- **CLAUDE.md** — Sluss-framing, SQLite-dev-flöde (Postgres/Redis/migrate/seed
  borta), riktiga svarsheaders, sensitivity-klasser + compliance-yta,
  MCP-surface.
- **AGENTS.md** — projektkontext till Sluss/control+evidence, orientering pekar
  på doc 13/08/09 istället för raderade 01/02, inga docker-compose-beroenden.
- **Arkitektur** (`02-context-diagram`, `03-request-lifecycle`,
  `05-low-latency`, `06-model-registry`, `07-policy-engine`, `09-data-model`,
  `10-api-contracts`, `14-deployment-topology`, `15-failure-modes`):
  Postgres/Redis-referenser ersatta med SQLite/in-memory-verkligheten;
  Enterprise 2.0-material tydligt märkt som framtid; Anthropic-/MCP-ytan
  dokumenterad; deployment-topologins självmotsägande övre halva ersatt;
  Redis-/Postgres-failure-modes ersatta med SQLite-volym-scenariot.
- **Engineering** (`01-routing-policy-reference` 8 sensitivity-klasser +
  rankningsregel; `03-classifier-design` V2-ML-avsnitt ersatt med
  deterministisk-by-design-beslutet; `04-verifier` superseded-not → council
  mode; `09-ci-cd` beskriver de två verkliga workflows; `10-local-development`
  omskriven från grunden — gamla var 100 % ogenomförbar).
- **Operations** (`metrics-catalog` omskriven till de metrics som faktiskt
  exporteras i `internal/metrics`; döda migrationsrader ur release-checklistorna).
- `docs/03-backlog/backlog-index.md` — sex trasiga epic-filnamn rättade.
- `db/README.md` — märkt "Enterprise 2.0, används inte i runtime".
- Varumärke Tokenizer→Sluss i filer som ändå redigerades + `docs/mcp.md`.

## Medvetet orört

- Alla 13 ADR:er — granskningen fann inget beslut motsagt av koden.
- EPIC-01–11, sprint-00–08, gamla issues, `docs/notes/` — korrekt historik.
- `docs/00-product/08–11` säger "tokenizer" i brödtext — strategidokument
  daterade före rebranden; innehållet är aktuellt, namnet är historiskt.
  (Live-URL:en tokenizer.froste.eu är faktisk tills DNS-flytten.)
- Lösa feature-docs (`autopilot`, `bandit`, `council`, `experiments`, `pii`,
  `api-keys`, `compliance-report`) — kodverifierade som korrekta.

## Acceptans

- [x] Ingen fil utanför historikkatalogerna beskriver Postgres/Redis som runtime
- [x] Inga döda kommandon/CLI:er i aktiva guider
- [x] `go build ./...` + testsvit gröna (docs + kommentarsändringar)
