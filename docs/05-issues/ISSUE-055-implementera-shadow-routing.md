# ISSUE-055: Implementera shadow routing

## Labels
- `epic: EPIC-08`
- `priority: P2`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera shadow routing som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Alternativ policy kan loggas utan execution.
- Shadow decision sparas.
- Dashboard kan jämföra.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- Chat-path har opt-in `ShadowPolicyCache` som beräknar alternativt policybeslut med samma engine/health-input men aldrig kör en extra provider-call.
- `eventlog.DecisionEvent` bär `ShadowComparison`; structured logs och ny in-memory tracker aggregerar actual-vs-shadow-diffar.
- `cmd/router` kan ladda shadow policy via `ROUTER_SHADOW_POLICY_PATH` och wirar comparison tracker i event fan-out.
- Dashboard JSON/HTML visar shadow summary/recent-diffar och behåller admin-only RBAC-skydd.
- Tester verifierar att primär provider kör exakt en gång och att shadow comparison sparas/exponeras.
