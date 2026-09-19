# ISSUE-034: Implementera minimal spend aggregation

## Labels
- `epic: EPIC-07`
- `priority: P0`
- `type: data`
- `sprint: 06`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera minimal spend aggregation som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Spend per modell aggregeras.
- Spend per tenant aggregeras.
- Actual usage används när tillgängligt.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
