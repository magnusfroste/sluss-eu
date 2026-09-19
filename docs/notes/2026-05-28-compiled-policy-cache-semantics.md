# 2026-05-28 - Compiled Policy Cache Semantics

## Context

ISSUE-022 added compiled in-memory policy snapshots and an atomic tenant/project cache on top of the Policy DSL v1 parser from ISSUE-021.

## What I Learned

Parser structs are not the same as fast-path policy semantics. The compiler must normalize backwards-compatible route hints into force/defaults/constraints and must preserve documented merge rules, otherwise the cache can be fast but subtly different from the DSL reference.

## Reuse Rules

- Open this note when changing policy compilation, reload, route matching, route hint handling, or constraint aggregation.
- Compile and validate every candidate policy before swapping the active cache.
- Keep cache lookups in-memory and scope-aware: project policy, then tenant policy, then default policy.
- Apply stricter constraint semantics during evaluation: allowed lists intersect, denied/capability lists union, max cost/latency keep the minimum, and `retention: none` wins over `standard`.
- Treat defaults as fill-only values. A catch-all default rule must not overwrite a value set by an earlier, more specific default rule.

## Failure Signals

- A route hint such as `tier`, `provider`, `verifier`, `timeout_ms`, or `require_capability` remains only in `Route.Hints` after compilation.
- Failed policy reload replaces or clears the last good active policy.
- Project-scoped cache entries are accepted without a tenant id.
- Multiple matching constraint rules append allowlists or raise cost/latency ceilings instead of narrowing them.
- A specific default such as `trivial_git -> cheap` is overwritten by the catch-all `when: {}` default.

## Next Checklist

- [ ] Re-read `06-engineering/01-routing-policy-reference.md` before changing compiled policy semantics.
- [ ] Add focused tests for hint normalization and multi-rule constraint merging.
- [ ] Include a policy test-runner case that proves specific defaults survive later catch-all defaults.
- [ ] Run `go test ./internal/policy` and `go test ./...`.
