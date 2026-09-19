# DB-backade API-nycklar (ISSUE-079)

Med `ROUTER_DATA_DIR` satt kan olika avdelningar få egna API-nycklar utan
omdeploy — LiteLLM-stil "virtual keys". Endast SHA-256-hashen lagras i
`history.db`; klartexten visas **en gång** vid skapande och kan aldrig hämtas
igen. Auth-vägen är cache-snabb: nycklarna laddas in i den in-memory-keystore
vid start och vid varje mutation — DB läses aldrig på request-vägen.

Env-nyckeln (`LOCAL_API_KEY`) finns kvar som bootstrap så en färsk deploy
aldrig låses ute.

## Admin-API (admin-gate: Bearer admin-nyckel eller dashboard-lösenord)

```bash
# Skapa nyckel för avdelningen "ekonomi" (klartexten visas EN gång):
curl -s -u admin:$ROUTER_DASHBOARD_PASSWORD -X POST \
  http://localhost:8080/router/keys \
  -d '{"tenant_id":"tn_ekonomi","project_id":"prj","role":"user"}'
# → {"key_id":"key_ab12...","key":"tk_....","warning":"store this key now — ..."}

# Lista (aldrig hemligheter):
curl -s -u admin:$ROUTER_DASHBOARD_PASSWORD http://localhost:8080/router/keys

# Återkalla:
curl -s -u admin:$ROUTER_DASHBOARD_PASSWORD -X DELETE \
  http://localhost:8080/router/keys/key_ab12...
```

## Vad en nyckel bär

Varje nyckel binds till `tenant_id` / `project_id` / `role` / `scopes`. Därmed
tänds allt som redan finns per avdelning automatiskt: spend-by-tenant,
budgettak, audit-actor, A/B-bucketing. Utan explicita scopes får nyckeln full
åtkomst (legacy-semantik); utan roll blir den `user`.

## Säkerhet

- **Hash i vila**: bara `sha256(nyckel)` lagras. Nycklarna är 256-bitars
  slumptokens (`tk_` + base64url) — osaltad SHA-256 är säker (rainbow tables
  gäller inte 256-bitars hemligheter) och en per-nyckel-salt skulle bryta den
  konstanta hash-map-uppslagningen auth-vägen bygger på.
- **Visas en gång**: mint-svaret är enda tillfället klartexten existerar.
- **Återkallning** är mjuk (rad kvar med `revoked_at` för audit) och evakuerar
  nyckeln ur keystore direkt → nekas omedelbart.
- **Auditerat**: mint/revoke skrivs till audit-sinken (aktör, key_id, tenant —
  aldrig hemligheten) och hamnar i den hash-kedjade exporten (ISSUE-075).

## Kvar (P2)

Live `last_used_at` (kräver keyID i auth-context + async batch) och per-nyckel
budget/rate limit. Kolumnen finns; fältet fylls i en följd-issue.
