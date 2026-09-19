# ISSUE-051: Implementera budget caps

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera budget caps som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Budget per tenant/project.
- Threshold warning.
- Block eller downgrade enligt policy.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-10)

Nytt paket `internal/budget`:

- **Budget per tenant/project**: `Caps` med per-tenant- och per-projekt-cap
  (projekt har företräde). `Ledger` ackumulerar spend i micro-USD per tenant och
  per tenant/projekt; implementerar `eventlog.Handler` (estimerad kostnad från
  decision-eventet) och wiras in på event-MultiHandlern i `cmd/router/main.go`.
- **Threshold warning**: `Evaluator.Check` returnerar `StatusWarn` vid
  `WarnThreshold` (default 0.8). Chat-handlern sätter `X-Router-Budget-Warning`.
- **Block eller downgrade**: `Cap.Action` (`block`|`downgrade`, default block).
  Över cap → handlern svarar `402 budget_exceeded` (block) eller tvingar
  `RouterModeCheap` + `X-Router-Budget-Action: downgrade` (downgrade).
- Wiring via `server.Config.Budget`; `ROUTER_BUDGET_USD` sätter en lokal
  default-cap. Checken är in-memory och ligger före routing → ingen extra
  latency på fast path. Caps är opt-in (ingen cap → `StatusOK`).
- Tester: warn/over-trösklar, block vs downgrade, projekt-företräde, default-vikt,
  ledger-ackumulering från events, samt handler-beteende (block/downgrade/warn).
