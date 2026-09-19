# ISSUE-043: Implementera provider allow/deny per projekt

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: security`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera provider allow/deny per projekt som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Policy kan begränsa providers.
- Override kan inte bryta denylist.
- Tester finns.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (status-sync)

Implementerat i commit `23f4fde` (feat(security): enforce provider allow/deny on
pinned models). Engine enforcar projektets allow/deny-listor och blockerar med
`provider_not_allowed` (se `internal/engine/engine.go`). Status-etiketten
justerad till `done` för att matcha koden.
