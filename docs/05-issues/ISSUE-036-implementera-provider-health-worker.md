# ISSUE-036: Implementera provider health worker

## Labels
- `epic: EPIC-07`
- `priority: P0`
- `type: backend`
- `sprint: 06`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera provider health worker som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Health score uppdateras.
- Error rate påverkar health.
- Router läser health cache.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
