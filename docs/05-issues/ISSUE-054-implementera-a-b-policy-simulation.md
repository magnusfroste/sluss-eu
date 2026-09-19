# ISSUE-054: Implementera A/B policy simulation

## Labels
- `epic: EPIC-08`
- `priority: P2`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera a/b policy simulation som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Två policies kan jämföras offline.
- Cost/route diff rapporteras.
- Ingen produktionspåverkan.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- Ny gemensam `engine.DecisionComparison` för deterministiska route/cost-diffar mellan två policybeslut.
- `internal/evals` kan köra samma dataset mot två kompilerade policies offline, utan provideranrop, och rapportera stabila ändringar per case.
- `cmd/eval-report` stöder `-policy-a`/`-policy-b` och skriver `comparison.json`/`comparison.txt`.
- Fixtures i `evals/policy-sim-*.yaml` täcker tenant-specifik rerouting och experiment-blockering.
