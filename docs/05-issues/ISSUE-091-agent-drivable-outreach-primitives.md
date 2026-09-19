# ISSUE-091 — Agent-drivbara outreach-primitiv på MCP-ytan (create + measure)

- `epic: EPIC-06`
- `priority: P2`
- `type: backend`
- `sprint: backlog`
- `state: done`

## Bakgrund (beslut 2026-07-09)

Lead-capture hör inte hemma i tokenizer (produkten installeras hos kund). CRM,
utskick och uppföljning sköts i **ett externt CRM**. Tokenizer blir "create + measure"-
backend: en agent (driven från ett externt CRM) ska via tokenizers admin-MCP kunna
skapa en per-prospekt-nyckel/länk och sedan hämta om den använts — så agenten
vet om CISOn faktiskt testat, utan att en människa loggar in i tokenizer.

Flödet agenten kör:
1. `mint_key` (eller `create_demo_link`) → per-prospekt-nyckel/länk med etikett.
2. ett externt CRM mailar CISOn + följer upp (utanför tokenizer).
3. `key_usage` / `demo_link_usage` → har den använts? hur mycket? senast när?
4. ev. `revoke_key` när testperioden är slut.

## Design — skriv-verktyg på MCP (medveten brytning mot read-only)

MCP-ytan har hittills varit **strikt read-only** (docs/mcp.md). Dessa verktyg
muterar → gate:as bakom **explicit opt-in** `ROUTER_MCP_WRITE_ENABLED=false`
(default av), utöver den befintliga admin-auth:en (Bearer admin-key /
dashboard-lösen). Varje mint/revoke auditeras (redan via `apikeys.Manager`).

Verktyg:
- `mint_key` — args: `label`/`department` (→ project), ev. `tenant`, `scopes`.
  Returnerar plaintext-nyckeln **en gång** + `key_id`. Wrappar `Manager.Mint`.
- `key_usage` — args: `key_id` (el. `project`/`label`). Returnerar `created_at`,
  `last_used_at`, `used` (bool), `request_count`, tokens, ev. första/senaste
  aktivitet. Byggstenar finns: `api_keys.last_used_at`, `history.ByTenant()`.
- `revoke_key` — args: `key_id`. Wrappar `Manager.Revoke`.
- (om namngivna demo-länkar byggs, ISSUE-089-rest) `create_demo_link` +
  `demo_link_usage` (opens/senast) — samma create+measure-mönster.

## Verifiering (att skriva)

Write-gate av/på (default av → verktygen registreras inte / returnerar fel),
mint→usage-roundtrip mot en in-memory store, revoke reflekteras, audit skrivs,
plaintext returneras bara vid mint.

## Non-goals

E-post/CRM/uppföljning (det är ett externt CRM). Kräver INTE att last_used_at redan är
perfekt wired — request-count per project räcker som "har testat"-signal; wira
last_used vid behov.

## Ersätter

Lead-form-delen av ISSUE-089 (den flyttar till ett externt CRM). Namngivna demo-länkar
+ "vem har provat"-statistik lever vidare här / i 089 som create+measure.
