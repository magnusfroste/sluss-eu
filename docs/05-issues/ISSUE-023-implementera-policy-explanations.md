# ISSUE-023: Implementera policy explanations

## Labels
- `epic: EPIC-04`
- `priority: P0`
- `type: backend`
- `sprint: 04`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera policy explanations som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Matched rules returneras.
- Explanations loggas.
- Header kan aktivera explain.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
