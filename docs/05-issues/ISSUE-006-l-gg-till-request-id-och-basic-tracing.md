# ISSUE-006: Lägg till request id och basic tracing

## Labels
- `epic: EPIC-07`
- `priority: P0`
- `type: backend`
- `sprint: 01`
- `category: enhancement`
- `state: done`

## Mål

Implementera lägg till request id och basic tracing som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Varje request får request id.
- Trace spans skapas.
- Request id returneras i header.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
