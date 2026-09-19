# ISSUE-003: Implementera API key auth skeleton

## Labels
- `epic: EPIC-09`
- `priority: P0`
- `type: security`
- `sprint: 01`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera api key auth skeleton som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Bearer token kan valideras mot hashad key.
- Ogiltig key ger 401.
- Tenant context skapas.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
