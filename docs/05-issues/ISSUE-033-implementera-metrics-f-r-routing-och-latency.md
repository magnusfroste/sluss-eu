# ISSUE-033: Implementera metrics för routing och latency

## Labels
- `epic: EPIC-07`
- `priority: P0`
- `type: backend`
- `sprint: 06`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera metrics för routing och latency som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Prometheus metrics finns.
- p95 kan mätas.
- Labels inkluderar modell och routeklass.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
