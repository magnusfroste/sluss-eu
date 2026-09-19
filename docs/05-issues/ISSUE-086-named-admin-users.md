# ISSUE-086: Namngivna konsol-användare (accountability)

## Labels
- `epic: EPIC-14`
- `priority: P1`
- `type: security`
- `sprint: sprint-09`
- `state: done` (2026-07-07)

## Bakgrund

Konsolen gick att logga in på med enbart ett delat lösenord
(`ROUTER_DASHBOARD_PASSWORD`). Auditkedjan tillskrev då allt `actor="admin"` —
ingen person. För en NIS2-accountability-pitch är det en lucka: en CISO vill
veta *vem* som ändrade policy, skapade en nyckel eller exporterade loggen.

## Levererat

- **DB-backade konton** (`admin_users` i history.db): användarnamn +
  PBKDF2-HMAC-SHA256-hash (stdlib, ingen ny dep; självbeskrivande format,
  utbytbart mot bcrypt/argon2), roll, disabled/last-login.
- **Login/session**: opaka slumpade cookies i minnet (12h TTL), login-sida,
  logga-ut. Guarden (`AdminAuth.Require`) accepterar session **eller** Bearer
  admin-nyckel (programmatisk åtkomst kvar) och redirectar annars till login.
- **Break-glass**: env-lösenordet loggar fortfarande in (som `... (break-glass)`)
  så man aldrig låser ut sig. Seed av första admin via `ROUTER_ADMIN_USER/
  ROUTER_ADMIN_PASSWORD`.
- **Users-adminsida** (`/router/users`): skapa/återställ, inaktivera/aktivera.
- **Audit-actor = inloggad användare** för nyckel-mint/revoke och
  användarändringar → auditen svarar "vem gjorde vad".
- Aktiveras när en data dir finns (samma mönster som nycklar/roster); utan data
  dir kvarstår legacy Basic Auth/Bearer.

## Icke-mål (senare)

SSO/OIDC (Entra/Okta), MFA, full RBAC-matris, inbjudningsflöden.

## Acceptanskriterier (uppfyllda)

- KDF hash/verify (konstant tid, slumpsalt); seed idempotent; login sätter
  session; guard redirectar utan session; break-glass funkar utan användare;
  inaktiverad användare nekas. `make demo-ciso` grön (bearer-admin-vägen).
