# ISSUE-047: Skapa CLI för decision debug

## Labels
- `epic: EPIC-10`
- `priority: P1`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa cli för decision debug som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- CLI kan anropa /router/decision.
- Visar selected model.
- Visar explanations.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-09)

- Ny CLI `cmd/routerctl` som postar en dry-run till `/router/decision` och
  skriver ut **selected model**, provider, policy-version, timeout, fallback-kedja
  och **explanations** (`decision_reasons`). `-explain` sätter `X-Router-Explain`.
- Flaggor: `-url`, `-key` (env `ROUTER_API_KEY`/`LOCAL_API_KEY`), `-model`,
  `-message`, `-explain`, `-stream`, `-timeout`.
- Klientlogiken (`fetchDecision`) är extraherad och testad mot `httptest`:
  lyckat beslut, policy-block (non-200 men giltigt beslut, inte fel),
  auth-fel (401) och `insufficient_scope` (403) som fel, samt rendering.
- Klienten avkodar bara den delmängd av `engine.RouteDecision`-kontraktet den
  renderar — illustrerar client-side contract decoding.
- Tillagd i `make build` (`bin/routerctl`).
