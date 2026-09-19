# EPIC-14: Nyckel-provisioning och per-avdelnings-accountability

## Mål

Gör API-nyckeln till accountability-enheten: generera/rotera/återkalla nycklar
i DB (LiteLLM-stil "virtual keys"), bind varje nyckel till tenant/projekt/
scopes/budget — så uppföljning, statistik och säkerhet per avdelning tänds
automatiskt i allt som redan finns (spend-by-tenant, budget, audit, A/B).

## Varför

Företag ger olika avdelningar olika nycklar — för uppföljning och säkerhet
(rotera en läckt nyckel utan att röra andra). Idag finns nycklar bara i env →
in-memory store; ingen provisioning utan omdeploy. PRD:
`.scratch/api-key-provisioning/PRD.md`.

## Scope

- Nyckeltabell i befintliga SQLite `history.db` (hash i vila; klartext visas
  en gång vid skapande)
- Admin-API: mint/list/revoke (admin-scope); env-nyckeln kvar som bootstrap
- Keys-adminsida i UI-navigationen
- Audit av mint/revoke (befintlig audit-sink)
- P2: per-nyckel-budget och rate limit

## Out of scope

- SSO/OIDC-integration (senare)
- Automatisk rotation/expiry-policy (P2)

## Acceptanskriterier

- Nycklar överlever omstart; auth-path förblir cache-snabb (in-memory läs,
  DB som källa).
- Ingen klartextnyckel i DB/loggar; secret visas exakt en gång.
- Mint/revoke är auditerade; demo med två "avdelningar" som får separata
  spend-rader på dashboarden.
