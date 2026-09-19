# ISSUE-038: Implementera eval runner

## Labels
- `epic: EPIC-08`
- `priority: P1`
- `type: backend`
- `sprint: 07`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera eval runner som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Kör evals mot flera modeller.
- Samlar cost och latency.
- Rapport genereras.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
