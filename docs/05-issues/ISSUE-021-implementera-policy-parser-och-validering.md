# ISSUE-021: Implementera policy parser och validering

## Labels
- `epic: EPIC-04`
- `priority: P0`
- `type: backend`
- `sprint: 04`
- `category: enhancement`
- `state: done`

## Intent

Implementera parser och validator för Policy DSL v1 från ISSUE-020. Outputen ska vara ett typed internt policykontrakt som senare kan kompileras av ISSUE-022 utan att göra schema- eller referenskontroller på fast path.

## Implementation Contract

- Parsern ska acceptera Policy DSL v1 i YAML och, om repo redan har JSON-ingress för config, samma struktur i JSON.
- YAML features som anchors, aliases och custom tags ska inte behövas eller uppmuntras. Om den valda parsern stödjer dem implicit ska valideringen fortfarande bara acceptera den dokumenterade datamodellen.
- Returnera en typed policystruktur med `version`, `metadata`, `settings` och ordnade `rules`.
- Validera obligatoriska top-level fält: `version`, `settings`, `rules`.
- Validera rule shape: unik `id`, obligatoriskt `when`, obligatoriskt `route`, och minst en route-effekt (`block`, `force`, `constraints`, `defaults` eller dokumenterad v1-hint).
- Validera vokabulär mot repo-kontrakten: `task_type`, `risk_level`, `router_mode`, `model_profile`, capabilities och retentionvärden.
- Validera referenser mot in-memory registry/profile metadata där det finns tillgängligt: okänd provider, modell, modellprofil eller capability ska ge tydligt valideringsfel.
- Validera glob-/matcherfält: tomma globbar, syntaktiskt trasiga patterns eller orimligt breda deny/allow-former ska få deterministiska fel.
- Validera policyprioritet utan att kompilera full runtime: block/force/constraints/defaults ska kunna representeras så ISSUE-022 kan bygga snabb utvärdering.
- Fel ska innehålla rule id, fältnamn/path och maskinsäkra error codes. Fel får inte dumpa prompttext eller hemligheter.
- Parser/validator ska köras vid policy load/admin, inte per request på fast path.

## Files / Packages

- Förväntad produktkod: `internal/policy` eller närliggande package för DSL types, parser och validator.
- Förväntad integration: policy load-kod eller tests kan skapa validator med registry/profile snapshot.
- Förväntade tester: table-driven parser/validator tests med valid defaultpolicy och invalid cases.
- Håll ändringen till Policy DSL parser/validation. Ingen routing evaluation i detta issue.

## Acceptance Criteria

- Valid defaultpolicy från ISSUE-020 parseas och valideras utan fel.
- Saknad `version`, `settings`, `rules`, rule `id`, `when` eller `route` stoppas.
- Duplicate rule ids stoppas.
- Okända `task_type`, `risk_level`, `router_mode`, capabilities, providers, models och model profiles stoppas när referensdata finns.
- Okända routefält eller conditionfält stoppas istället för att ignoreras tyst.
- Regler som refererar till klientoverride dokumenteras/valideras så de inte kan bryta säkerhets- eller tenantpolicy.
- Valideringsfel är strukturerade nog för API/CLI-output och tester.

## Tests / Verification

- Unit test: komplett valid YAML-policy parseas till typed struktur med bibehållen rule order.
- Unit test: JSON-ekvivalent policy parseas om JSON-ingress stöds.
- Unit test: duplicate rule id ger deterministiskt fel med rätt rule id.
- Unit test: unknown model/provider/profile/capability ger valideringsfel.
- Unit test: invalid glob/pattern ger valideringsfel.
- Unit test: unknown field i `when` eller `route` stoppas.
- Unit test: felpayload innehåller inte rå prompttext eller secrets.
- Kör fokuserade Go-tester för policypaketet.

## Out of Scope

- Ingen compiled policy cache eller hot reload.
- Ingen runtime policy evaluation i request path.
- Ingen DB-lagring av policies.
- Ingen admin UI eller policy test runner.

## Dependencies

- ISSUE-020 för DSL-kontrakt.
- ISSUE-010 och registry/profile metadata för modell- och capabilityvalidering.
- `01-architecture/07-policy-engine.md`
- `01-architecture/05-low-latency-architecture.md`

## Subagent Notes

- Gör parsern strikt. Tyst ignorering av okända fält är farligt i policykod.
- Separera parsefel från valideringsfel så CLI/admin senare kan ge bättre output.
- Håll valideringen load-time only; request path ska bara läsa kompilerad policy.

## Klar när

- Policy DSL v1 kan parseas och valideras strikt med tydliga fel.
- Okända referenser stoppas före policy activation.
- Acceptance criteria och fokuserade tester passerar.
