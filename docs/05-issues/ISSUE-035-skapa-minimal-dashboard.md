# ISSUE-035: Skapa minimal dashboard

## Labels
- `epic: EPIC-07`
- `priority: P0`
- `type: frontend`
- `sprint: 06`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa minimal dashboard som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Dashboard visar spend.
- Visar route distribution.
- Visar latency p95.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
