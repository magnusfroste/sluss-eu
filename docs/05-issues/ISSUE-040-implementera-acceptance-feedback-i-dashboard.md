# ISSUE-040: Implementera acceptance feedback i dashboard

## Labels
- `epic: EPIC-08`
- `priority: P1`
- `type: frontend`
- `sprint: 07`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera acceptance feedback i dashboard som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Outcomes visas.
- Acceptance rate per modell visas.
- Filtrering per taskklass.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
