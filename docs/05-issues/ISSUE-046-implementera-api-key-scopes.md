# ISSUE-046: Implementera API key scopes

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: security`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera api key scopes som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Scopes definieras.
- Endpoints kräver rätt scope.
- Tester finns.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-09)

- **Scopes definieras**: konstanter i `internal/auth` — `chat:completions`, `router:decision`, `router:outcomes` (matchar `api_keys.scopes` i schema/seed) samt wildcard `tenant.ScopeWildcard` (`*`). `tenant.Tenant` har nu ett `Scopes`-fält och en `HasScope`-metod (tom mängd = obegränsad legacy-nyckel; `*` = allt).
- **Endpoints kräver rätt scope**: ny `auth.RequireScope`-middleware som körs efter `auth.Middleware` och returnerar `403 insufficient_scope` (OpenAI-feltkuvert) vid saknad scope. Wirad i `server.New`: `/v1/chat/completions` → `chat:completions`, `/router/decision` → `router:decision`, `/router/outcomes` → `router:outcomes`. Saknad scope loggas strukturerat.
- Lokal nyckel (`cmd/router/main.go` + `db/seeds/local.sql`) provisioneras med alla tre scopes.
- **Tester**: `internal/tenant` (HasScope-matris, nil-säkerhet), `internal/auth` (middleware tillåter/nekar, obegränsad nyckel, saknad tenant), `internal/server` (end-to-end 403 på chat-endpoint för scope-begränsad nyckel).
- Enforcement sker i auth-lagret före routing → ingen påverkan på fast-path-latency.
