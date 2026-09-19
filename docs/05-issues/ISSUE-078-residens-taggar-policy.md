# ISSUE-078: Dataresidens/compliance-taggar i rostern + policyvillkor

## Labels
- `epic: EPIC-13`
- `priority: P0`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-05)

## Mål

Gör dataresidens och leverantörs-compliance till **förstklassiga policyvillkor**:
tagga providers/modeller med compliance-metadata (data, inte kod) och låt
policyn kräva/förbjuda taggar per task/risk/sensitivity —
*"personuppgifter får bara gå till EU-residenta, DPA-godkända leverantörer"*.

## Design

- **Roster-metadata**: `compliance_tags: []string` på provider och modell i
  SQLite-rostern (ISSUE-073) + seed-datan. Exempel-taggar: `eu-resident`,
  `dpa-signed`, `iso27001`, `us-only`. Modell ärver providerns taggar och kan
  addera egna.
- **Policyvillkor** i `route.constraints`:
  - `require_provider_tags: [eu-resident, dpa-signed]` — kandidat utan alla
    taggar filtreras bort
  - `deny_provider_tags: [us-only]`
- Filtreringen sker i `FilterCandidates`/`providerModelConstraintReason` —
  samma enda sanningskälla som allow/deny-listorna, så pinned-model-vägar kan
  inte kringgå den.
- Exkluderingsskäl blir läsbara: `"provider openrouter saknar tag eu-resident"`
  → syns i decision reasons, `/router/decision` och `route_explain` (MCP).
- Redigerbart på Providers/Models-adminsidorna (textfält, kommaseparerat).

## Acceptanskriterier

- Golden policy-cases: (1) sensitivity=personal + require eu-resident →
  icke-taggad provider exkluderas; (2) ingen kandidat kvar → begripligt
  no_route/block, inte tyst fallback till fel leverantör.
- Force/pinned model kan **inte** kringgå deny/require-taggar (test).
- Taggar syns i compliance-rapporten (ISSUE-076, sektion Kontrollstatus).
- Docs: policy-referensen §6.3 uppdaterad med tagg-villkoren.
