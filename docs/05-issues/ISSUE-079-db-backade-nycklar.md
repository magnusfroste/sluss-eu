# ISSUE-079: DB-backade API-nycklar v1 (mint/revoke, hash i vila)

## Labels
- `epic: EPIC-14`
- `priority: P0`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-05)

## Mål

Generera, lista och återkalla API-nycklar via admin-API, persisterade i
`history.db` — så olika avdelningar får egna nycklar (tenant/projekt/scopes)
utan omdeploy. LiteLLM-stil "virtual keys".
PRD: `.scratch/api-key-provisioning/PRD.md`.

## Design

- **Tabell** `api_keys` i history.db: `key_id, key_hash (sha256+salt),
  tenant_id, project_id, role, scopes, created_at, revoked_at, last_used_at`.
  Klartextnyckeln (`tk_<slumpad 32B base62>`) returneras **exakt en gång** vid
  skapande; endast hash lagras.
- **Admin-API** (admin-scope, samma auth-mönster som övriga adminytor):
  - `POST /router/keys` `{tenant_id, project_id, role, scopes}` → `{key_id, key}`
  - `GET /router/keys` → lista (utan hemligheter)
  - `DELETE /router/keys/{key_id}` → revoke (soft, `revoked_at`)
- **Auth-pathen**: keystore läser DB vid boot + vid mutation (cache i minnet;
  ingen DB-läsning per request). Env-nyckeln (`LOCAL_API_KEY`) kvar som
  bootstrap/admin så en färsk deploy aldrig låses ute.
- **Audit**: mint/revoke via befintlig audit-sink (actor, key_id, tenant —
  aldrig hemligheten). Kedjas i ISSUE-075-exporten.
- `last_used_at` uppdateras lazily/asynkront (ej på hot path per request —
  batcha eller uppdatera högst var N:e minut).

## Acceptanskriterier

- Nycklar överlever omstart; revoked nyckel nekas direkt efter mutation.
- Ingen klartextnyckel i DB, loggar eller audit; visas en gång i svaret.
- Auth-latens oförändrad (in-memory cache; bench före/efter).
- Tester: mint→auth ok; revoke→401; hash-verifiering; bootstrap-nyckel kvar;
  audit-poster skrivs.
- Demo: två nycklar ("ekonomi", "utveckling") → separata spend-rader.
