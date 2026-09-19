# ISSUE-060: Implementera global conservative mode

## Labels
- `epic: EPIC-05`
- `priority: P1`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera global conservative mode som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Feature flag finns.
- Okända tasks routeas premium/balanced enligt policy.
- Incidentrunbook refererar till flaggan.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-11)

- **Feature flag**: `Engine.SetConservative(bool)` / `Conservative()` (atomisk,
  runtime-togglebar), wirad från env `ROUTER_CONSERVATIVE_MODE` i `cmd/router/main.go`.
- **Okända tasks routeas premium/balanced enligt policy**: `JobDescriptor` bär nu
  `TaskConfidence` (från classifiern). När flaggan är på markeras osäkra requests
  (`TaskUnknownHighRisk` eller confidence < 0.5) som `Conservative`, och
  `MinimumTierForTask` höjer golvet till minst `balanced`. Höjningen sänker aldrig
  ett starkare golv — policy-/task-forcing till `premium` (t.ex. security_review)
  gäller fortfarande. Filter + scoring delar samma golv så billiga modeller hård-
  filtreras bort.
- **Incidentrunbook**: nytt avsnitt i `07-operations/runbook.md` som refererar
  flaggan och stegen för att aktivera/verifiera/stänga av.
- Tester: floor-höjning för låg-confidence, ingen nedgradering av premium,
  flagg-gating per confidence i `Decide`, samt end-to-end att osäkra requests
  väljer ≥ balanced. `MinimumTierForTask` tar nu `*JobDescriptor` (call sites +
  tester uppdaterade).
