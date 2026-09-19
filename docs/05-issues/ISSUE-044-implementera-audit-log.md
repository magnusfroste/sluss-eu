# ISSUE-044: Implementera audit log

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: security`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera audit log som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Policy changes loggas.
- API key changes loggas.
- Blockerade requests loggas.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-09)

- Nytt paket `internal/audit`: `Entry`, `Action`, `Sink` med nil-säker `Record`-hjälpare. Sinks: `LogSink` (strukturerad logg, deterministisk), `MemorySink` (bounded ring buffer för in-process retrieval/test) och `MultiSink` (fan-out, speglar `eventlog.MultiHandler`).
- **Policy changes**: `policy.Cache.SetAuditor` + audit i `Reload` — en post per scope vid lyckad omladdning (target = policy-version) och en `failure`-post vid avvisad/ogiltig policy.
- **API key changes**: `auth.InMemoryKeyStore.SetAuditor`, audit i `Add` samt ny `Disable`-metod (`api_key.add` / `api_key.disable`).
- **Blockerade requests**: `ChatOptions.Auditor` + `auditBlocked` i chat-handlern på `ErrBlocked`-grenen (`request.blocked`, outcome `blocked`, bär request-id, tenant/projekt, blockkod och pinnad modell).
- Wiring i `cmd/router/main.go` (LogSink + MemorySink via MultiSink) och `server.Config.Auditor`.
- Persistens-schema: `db/migrations/003_audit_log.sql` (append-only, outcome-check, index per tenant/action/request — retention hanteras av ISSUE-045).
- Inga prompttexter eller secrets loggas; audit ligger utanför fast path och påverkar inte routingbudgeten.
