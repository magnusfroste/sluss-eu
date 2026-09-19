# 2026-06-12 - RBAC Role Compatibility Boundaries

## Context

ISSUE-057 added an RBAC skeleton on top of the existing API-key auth model. The repo already used scopes for request APIs, while dashboard endpoints had no auth gating.

## What I Learned

Roles and scopes need separate checks so admin-only control-plane surfaces do not accidentally change the behavior of chat, decision, or outcome APIs.

During a staged rollout, the zero-value role needs an explicit compatibility policy. Treating an unset role as legacy-unrestricted preserved existing API key behavior while still letting explicit `user` keys fail closed on admin-only dashboard routes.

## Reuse Rules

- Keep role enforcement on explicitly admin-only routes; do not overload scope middleware with role semantics.
- Model the compatibility rule in one place on the authenticated principal, then reuse it from middleware.
- Preserve the existing empty-scope legacy behavior independently from role checks.
- Add route-level tests that cover `admin`, `user`, legacy-unset role, and missing bearer token cases.

## Failure Signals

- A `user` role key can load `/router/dashboard` or `/router/dashboard/data`.
- A key with the correct scope starts failing chat or decision requests after RBAC changes.
- Legacy keys with no role suddenly lose access to previously available surfaces during rollout.
- Role names start appearing inside scope lists to work around missing role checks.

## Next Checklist

- Verify role constants and compatibility rules live on the tenant principal, not scattered through handlers.
- Verify admin-only routes return `insufficient_role`, not `insufficient_scope`.
- Re-run focused auth/server/tenant tests with explicit `user`, `admin`, and legacy keys.
