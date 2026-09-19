# ISSUE-081: Dashboard — "Egress & Compliance" + "Learning & routing"

## Labels
- `epic: EPIC-15`
- `priority: P1`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Mål

Två nya dashboardsektioner som gör (1) egress-kontrollen och (2) Fas 4-lärandet
**synligt** — idag finns datan bara i headers/CLI/loggar. Sektionerna talar
revisorspråk först, modell-slugs sen.

## Sektion "Egress & Compliance"

- Räknare (period): blockerade requests per block-orsak, masking-events,
  PII-träffar per typ (från ISSUE-077; typ + antal, aldrig värden),
  residens-exkluderingar (från ISSUE-078).
- Egress-tabell: provider → region/taggar → requests → spend (vart lämnade
  data huset).
- Incident-spakar: conservative mode på/av, öppna circuits (explicit, inte
  bara health 0.00), budget-status per tenant.
- Länk: "Generera kontrollrapport" → ISSUE-076-endpointen.

## Sektion "Learning & routing"

- Bandit-armar: per taskklass topp-modell (pulls, mean reward) via
  `Bandit.Snapshot()`.
- A/B: requests per arm (control/treatment) + spend per arm (från
  `ExperimentArm` på decision events).
- Council: antal paneler, genomsnittlig agreement, lågt-agreement-flaggor.

## Design

- Utöka `DashboardData` + template med de två sektionerna; datakällor finns
  (health/circuit, audit/räknare, history, bandit, eventlog). Räknare som inte
  redan aggregeras läggs i history-store (samma mönster som spend).
- Tenant-panelerna ("Tenants", "Spend by tenant") flyttas in under Egress &
  Compliance-kontexten (per-avdelningsvyn) i stället för att tas bort —
  beslutet är att tenants är centrala för uppföljning.
- Ärliga tom-tillstånd ("Inga blockeringar ännu") — inga fejkade siffror.

## Acceptanskriterier

- Sektionerna renderas med demo-data och med tomma data.
- JSON-payloaden (`/router/dashboard/data`) exponerar samma fält (för MCP/BI).
- Handler-test på nya fält; inga nya hot-path-beroenden.
