# ISSUE-028: Implementera /router/decision dry-run

## Labels
- `epic: EPIC-05`
- `priority: P0`
- `type: backend`
- `sprint: 05`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera /router/decision dry-run som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Endpoint returnerar route utan provideranrop.
- Visar kostnadsestimat.
- Visar explanations.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
