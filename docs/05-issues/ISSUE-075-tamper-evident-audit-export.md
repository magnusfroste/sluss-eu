# ISSUE-075: Tamper-evident audit-export (hash-kedja) + verify-CLI

## Labels
- `epic: EPIC-12`
- `priority: P0`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-05)

## Mål

Gör audit-trailen **överlämningsbar och manipulationssäker**: exportera
auditposter som hash-kedjad JSONL (varje post bär `prev_hash`; kedjan börjar i
en genesis-post) och ett CLI/underkommando som verifierar kedjan offline.
En revisor ska kunna få filen och själv bevisa att inget ändrats/tagits bort.

## Bakgrund

Audit-sinken (ISSUE-044) loggar redan block, nyckelhändelser och policybyten
strukturerat (LogSink + MemorySink). Gapet är integritet + export: idag är det
loggrader, inte ett bevis. Hash-kedjan är enkel, beroendefri och räcker för
tamper-evidens (märk: evidens, inte förhindrande).

## Design

- Ny `audit.ChainSink`: wrappar en sink, berikar varje Entry med
  `seq`, `entry_hash = sha256(canonical_json)`, `prev_hash`.
  Persistens till fil under `ROUTER_DATA_DIR` (`audit-chain.jsonl`, append-only)
  eller tabell i `history.db` — välj det som är enklast givet ISSUE-070.
- Export: `GET /router/audit/export` (admin-scope) streamar JSONL.
- Verify: `router audit verify <fil>` (eller `cmd/audit-verify`) — går igenom
  kedjan, exit != 0 + tydlig rad vid brott.
- Asynkront/utanför hot path som övrig audit; fel här får aldrig påverka
  requesten.

## Acceptanskriterier

- Manipulation (ändrad/borttagen/inskjuten rad) upptäcks av verify med radnr.
- Export kräver admin; ingen råpromptdata i posterna (maskingprincipen).
- Enhetstester: kedjebygge, verify-happy-path, tre manipulationsfall.
- Runbook-avsnitt: hur man tar ut och verifierar en export.
