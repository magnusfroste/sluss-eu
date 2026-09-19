# ISSUE-025: Implementera candidate filtering

## Labels
- `epic: EPIC-05`
- `priority: P0`
- `type: backend`
- `sprint: 05`
- `category: enhancement`
- `state: done`

## Intent

Implementera deterministic candidate filtering mellan policy evaluation och scoring. Kandidatsetet ska starta från in-memory registry/profiles och minska utifrån capabilities, tenant/project constraints, budget/latency hints och provider health innan ISSUE-026 räknar score.

## Implementation Contract

- Input ska vara `JobDescriptor`, compiled/evaluated policy constraints, registry/profile snapshot och provider health snapshot.
- Output ska vara en ordnad lista av kandidater plus structured reasons för bortfiltrerade kandidater.
- Filtering ska ske före scoring och utan DB, nätverk, LLM eller live provider metadata.
- Starta från registryns aktiva model/provider mappings. Disabled/deprecated modeller ska inte bli kandidater om registry markerar dem otillgängliga.
- Applicera policy block/allow/deny först: denied provider/model vinner över allowlist-hint, och blockerat beslut ska inte skapa kandidater.
- Applicera required capabilities: `streaming`, `tool_use`, `json_schema`, `vision`, `long_context`, samt domain strengths som code/security där registry stödjer dem.
- Applicera request/task krav: streaming requests kräver streaming-capable adapter; tool calls kräver tool capability; JSON schema/response_format kräver JSON schema support.
- Applicera model profile/tier constraints: forced `premium` får inte sänkas till `balanced` eller `cheap`; conservative unknowns får inte välja underpowered candidate.
- Applicera provider health från snapshot: hard-down providers exkluderas, degraded providers kan behållas med reason/penalty om policy tillåter och scoring senare väger dem.
- Definiera no-candidates behavior: returnera typed error/decision reason som routing kan översätta till 4xx/5xx eller fallback enligt policy, inte panic eller tom implicit default.
- Bevara deterministic ordering så samma input ger samma kandidatordning före scoring.

## Files / Packages

- Förväntad produktkod: `internal/router` eller närliggande package för candidate filtering.
- Förväntad integration: registry snapshot från Sprint 02, compiled policy output från ISSUE-022, route decision flow inför ISSUE-026.
- Förväntade tester: table-driven unit tests med small fake registry/policy/health snapshots.
- Håll ändringen till candidate filtering och reasons. Själva scoreformeln hör till ISSUE-026.

## Acceptance Criteria

- Capabilities filtrerar bort modeller/providers som inte kan uppfylla requesten.
- Policy allowed/denied providers och models appliceras före scoring.
- Forced model profile/tier respekteras och sänks inte av billigare kandidater.
- Provider health kan exkludera hard-down provider utan live lookup.
- Degraded provider hanteras deterministiskt enligt kontraktet.
- No-candidates case returnerar explicit typed result/error med reasons.
- Filterreasons kan loggas/debuggas utan prompttext.

## Tests / Verification

- Unit test: tool request filtrerar bort candidates utan `tool_use`.
- Unit test: JSON schema request filtrerar bort candidates utan `json_schema`.
- Unit test: tenant denies provider A även om provider A annars bäst matchar.
- Unit test: forced premium profile utesluter cheap/balanced candidates.
- Unit test: hard-down provider exkluderas från health snapshot.
- Unit test: no candidates returns explicit error/result and reason list.
- Unit test: repeated same input returns stable order.
- Kör fokuserade Go-tester för router/filterpaketet.

## Out of Scope

- Ingen scoreberäkning eller final model selection.
- Ingen fallback chain construction.
- Ingen provider health worker.
- Ingen DB-backed registry lookup på request path.

## Dependencies

- ISSUE-009/010 för registry och model profiles.
- ISSUE-014 för `JobDescriptor`.
- ISSUE-020-022 för policy constraints.
- ISSUE-036 för mer komplett provider health senare.
- `01-architecture/04-routing-engine.md`
- `01-architecture/08-provider-abstraction.md`

## Subagent Notes

- Gör reasons förstklassiga. Candidate filtering blir svårt att debugga utan tydliga "excluded_by_policy", "missing_capability" och "provider_unhealthy".
- Undvik att scoring smygstartar i detta issue. Filtrering ska avgöra eligibility, inte ranka kvalitet.
- Testa med små snapshots så beteendet blir lätt att läsa.

## Klar när

- Candidate filtering är deterministic, policy-aware, capability-aware och health-aware.
- No-candidates behavior är explicit och testat.
- Acceptance criteria och fokuserade tester passerar.
