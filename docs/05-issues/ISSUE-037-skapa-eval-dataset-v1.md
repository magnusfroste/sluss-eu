# ISSUE-037: Skapa eval dataset v1

## Labels
- `epic: EPIC-08`
- `priority: P1`
- `type: product`
- `sprint: 07`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa eval dataset v1 som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Minst 50 evalfall.
- Taskklass och expected route definierad.
- Inga verkliga secrets.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
