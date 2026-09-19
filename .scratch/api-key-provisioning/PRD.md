# PRD: API-key provisioning (DB-backed, per-department keys)

Status: backlog / not started (captured 2026-07, deferred by owner — "det tar vi senare")

## Why

A company running tokenizer as a shared model router wants **different keys for
different departments** (and teams/apps). Keys are the unit of:

- **Uppföljning & statistik** — attribute spend, request volume, model mix, and
  outcomes per department. This already flows through the `tenant`/`project`
  model (spend-by-tenant, budgets, audit) — the missing piece is *creating* keys.
- **Säkerhet** — scope/role per key, rotate/revoke a leaked key without touching
  others, and (later) per-key rate limits and budgets.
- **Flexibilitet över tid** — the same tenant can mint several keys used
  differently (prod vs staging, per-app, per-department), each with its own
  policy scope and budget.

## Current gap

Today keys live **only in env** (`LOCAL_API_KEY`) and are loaded into an
in-memory `auth.InMemoryKeyStore` at boot. That means:

- No way to create/rotate/revoke a key without a redeploy.
- One seeded key = one tenant (`tn_local`). Multi-department is not exercised.

## Direction (LiteLLM-style "virtual keys")

LiteLLM **generates** keys and persists them in a **DB**, not env. Each key row
carries: tenant/team, scopes/role, optional budget, optional rate limit,
metadata, created/last-used, and revoked flag. Borrow that shape:

1. **Durable key store** — persist keys in the existing SQLite `history.db`
   (the roster already lives there — same store, new table), so keys survive
   restarts. Env key stays as a bootstrap/admin key.
2. **Provisioning API + admin UI** — `POST /router/keys` (admin-scoped) to mint a
   key → returns the secret **once**; store only a hash. List/revoke/rotate.
   A "Keys" page in the admin nav.
3. **Key → tenant/project/scopes/budget** binding — so attribution, RBAC,
   per-tenant budget caps, and A/B bucketing (already keyed by tenant+project)
   all light up per department automatically.
4. **Security** — store only a salted hash of the secret; show the plaintext
   once at creation; support revoke + rotate; audit every mint/revoke (the audit
   sink already exists).

## Notes / non-goals for v1

- Reuse what's already built: tenants, projects, scopes/roles, budgets, audit,
  spend-by-tenant — this feature is mostly the *provisioning + persistence*
  layer on top.
- Per-key rate limiting can come after per-key budgets.
- Keep the env bootstrap key so a fresh deploy is never locked out.

## Owner note

Explicitly deferred. Recorded so it is not lost: companies will use distinct
keys per department for follow-up/statistics and security, and keys may be used
in different ways over time.
