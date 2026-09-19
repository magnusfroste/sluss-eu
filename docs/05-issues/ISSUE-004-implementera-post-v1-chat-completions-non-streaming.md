# ISSUE-004: Implementera POST /v1/chat/completions non-streaming

## Labels
- `epic: EPIC-01`
- `priority: P0`
- `type: backend`
- `sprint: 01`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera post /v1/chat/completions non-streaming som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Endpoint accepterar OpenAI-liknande request.
- Request proxas till mock provider.
- Response normaliseras.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.
