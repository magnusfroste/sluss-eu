# ISSUE-041: Skapa regression suite för felrouting

## Labels
- `epic: EPIC-08`
- `priority: P1`
- `type: test`
- `sprint: 07`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa regression suite för felrouting som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Incidentfall kan läggas till.
- CI kör regression.
- Expected routes valideras.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
