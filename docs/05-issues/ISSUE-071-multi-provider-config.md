# ISSUE-071: Multi-provider — config-driven endpoints, keys by env-ref

## Labels
- `epic: EPIC-11`
- `priority: P1`
- `type: enhancement`
- `sprint: v1.0`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Låt admin lägga till fler modell-providers (OpenAI-kompatibla endpoints) utan kodändring, med **secrets kvar i env**. En provider = icke-hemlig endpoint-config (namn, base URL, vilken env-var som håller nyckeln, vilka modeller/tiers/priser) + en nyckel som refereras per namn och sätts i miljön.

Konkret första fall: **z.ai** — `base_url=https://api.z.ai/api/coding/paas/v4`, nyckel via `ZAI_API_KEY`, en balanced-modell `glm-4.6`.

## Bakgrund

I dag byggs providern i `cmd/router/main.go`: en `OpenAIAdapter` mot OpenRouter, registrerad som provider `openrouter`, med registret hårdkodat i `internal/registry/openrouter.go`. `registry.Provider` har redan `BaseURL` + `AuthSecretRef` (env-var-namn) — mönstret är förberett. Providers-sidan (ISSUE: byggd) visar redan providers, nyckel-status (per env-var-namn, aldrig värdet) och modeller.

**Säkerhetsprincip (beslutad):** API-nycklar sätts i env/secret-store, aldrig i SQLite/UI. UI/config hanterar endast icke-hemlig endpoint-config. Detta bevarar CISO-positioneringen (secrets utanför appen/DB).

## Acceptanskriterier

- En deklarativ provider-config (JSON- eller YAML-fil via `ROUTER_PROVIDERS_CONFIG`, alternativt en JSON-array i env) som beskriver extra providers:
  ```json
  [{"id":"zai","name":"Z.ai","base_url":"https://api.z.ai/api/coding/paas/v4",
    "key_env":"ZAI_API_KEY",
    "models":[{"id":"zai-coder","tier":"balanced","provider_model_id":"glm-4.6",
      "input_usd_per_mtok":"0.6","output_usd_per_mtok":"2.2","capabilities":["chat"]}]}]
  ```
- Vid start: för varje provider byggs en `OpenAIAdapter` (BaseURL + nyckel från `key_env`), providern + dess modeller läggs till registry-definitionen (Provider med `AuthSecretRef=key_env`), och adaptern registreras under `id`. Saknad nyckel → providern läggs till men markeras (Providers-sidan visar redan "⚠️ nyckel saknas") och dess modeller filtreras bort från routing tills nyckeln finns.
- Routing kan välja dessa modeller via befintlig tier-logik (de deltar i scoring som vilken modell som helst). Ingen nyckel loggas eller persisteras.
- Providers-sidan visar de nya providerna (redan generiskt byggd — verifiera).
- Bakåtkompatibelt: utan config beter sig allt som i dag (OpenRouter via `OPENROUTER_API_KEY`).
- (Senare, separat issue) UI-CRUD för endpoint-config i Providers-sidan som skriver till `kv`/config — men fortfarande nycklar-per-env-ref. Detta issue levererar config-driven; UI-редigering är uppföljning.

## Tekniska noter

- Återanvänd `OpenAIAdapter` (z.ai är OpenAI-kompatibelt). Timeout/headers som OpenRouter-adaptern.
- Bygg registry-modeller med rimliga defaults (Capabilities.Chat=true, quality-priors per tier, ContextWindow) + priser från config (jfr `cost.USDPerMillion`).
- Slå ihop med roster-override (ISSUE-068): override sätter ProviderModelID på befintliga tier-modeller; detta issue *lägger till* modeller/providers. Dokumentera hur de samverkar.
- Håll fast-path oförändrad; providerbygget sker bara vid start.

## Klar när

- z.ai kan läggas till via config + `ZAI_API_KEY`, syns på Providers-sidan med nyckel-status, och kan routas till. Full svit grön; `make smoke` opåverkad.
- `.env.example` + `deploy/example.env` dokumenterar `ROUTER_PROVIDERS_CONFIG` (och att nycklar sätts separat i env).

## Härkomst

Enterprise-inriktning: skala från en hårdkodad provider till många, med secrets kvar i env. Providers-sidan byggd som grund; detta issue gör den config-driven. Beslut om lagringstrappa: DECISION_LOG 2026-07-04.
