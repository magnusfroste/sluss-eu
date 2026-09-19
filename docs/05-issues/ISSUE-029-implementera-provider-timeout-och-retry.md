# ISSUE-029: Implementera provider timeout och retry

## Labels
- `epic: EPIC-06`
- `priority: P0`
- `type: backend`
- `sprint: 05`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera provider timeout och retry som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Timeout per attempt stöds.
- Retryregler implementerade.
- Fel loggas.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
