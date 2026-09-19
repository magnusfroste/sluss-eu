# ISSUE-005: Bygg mock provider

## Labels
- `epic: EPIC-06`
- `priority: P0`
- `type: test`
- `sprint: 01`
- `category: enhancement`
- `state: done`

## Mål

Implementera bygg mock provider som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Mock kan returnera normal response.
- Mock kan simulera timeout och 429.
- Mock stödjer token usage.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
