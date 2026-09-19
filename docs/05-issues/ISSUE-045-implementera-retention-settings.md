# ISSUE-045: Implementera retention settings

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: data`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera retention settings som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Promptlogging kan stängas av.
- Retention per tenant.
- Cleanup job finns.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-09)

Nytt paket `internal/retention`:

- **Retention per tenant**: `Settings` med global default (`DefaultRetentionDays = 30`, matchar `tenants.retention_days`) + per-tenant overrides via `SetTenant`. `RetentionDays`/`Cutoff` är lock-fria och säkra på request-path.
- **Promptlogging kan stängas av**: `Settings.PromptLoggingEnabled(tenant)` — **av som default**, kan slås på globalt (`ROUTER_PROMPT_LOGGING`) eller per tenant, och stängas av per tenant. Konsumeras av `ChatOptions.logPrompt` i chat-handlern; innehåll körs genom secret-masking och loggas på debug-nivå.
- **Cleanup job**: `Cleaner.Sweep`/`Run` bygger en purge-plan per tabell — per-tenant sweep för overrides + en default-sweep som exkluderar dem; tabeller utan `tenant_id` (route_attempts) sweepas tidsbaserat. `Purger`-interface med `SQLPurger` (parametriserade DELETEs mot valfri `database/sql`-Execer) och `DryRunPurger` (loggar utan att radera). Fel räknas och loggas utan att avbryta sweepen.
- **Worker**: `cmd/worker` är inte längre en no-op — den kör en periodisk retention-sweep (dry-run tills en riktig DB-Execer wiras in).
- Wiring i `server.Config.Retention` + `cmd/router/main.go`; env-varor i `.env.example`.
- Inga prompttexter loggas utan explicit opt-in; retention-logiken ligger utanför fast path.
