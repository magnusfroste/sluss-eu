# ISSUE-009: Implementera in-memory model registry

## Labels
- `epic: EPIC-02`
- `priority: P0`
- `type: backend`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Build the first in-memory model registry package used by the router fast path. The registry must expose immutable snapshots of configured models/providers so routing can resolve models, providers, capabilities, and costs without database reads or external lookups per request.

The registry is the source for `registry_version`/`model_registry_version` on routing decisions and must support reload by swapping the active snapshot atomically.

## Implementation Contract

- Add a registry component that loads model/provider metadata at startup from the existing MVP source selected by the repo, such as static config/YAML or repository-local seed data.
- Represent the active registry as an immutable snapshot. Request handling may read from the active snapshot but must not mutate it.
- Every snapshot exposes a stable `registry_version` string and creation timestamp.
- Support lookup by internal model id, provider id, and provider model id where useful for adapter dispatch.
- Support model/provider lookup without database reads, HTTP calls, LLM calls, or other external service calls on the fast path.
- Add capability filtering helpers for requirements such as `chat`, `streaming`, `tool_calls`, `json_schema`, `vision`, and `long_context`.
- Add enabled/disabled filtering so disabled models are never selected by normal routing.
- Keep static registry data separate from dynamic health fields. Health may be layered later, but this issue must not mix mutable health state into model metadata.
- Add reload behavior that validates a candidate registry, builds a new immutable snapshot, then atomically swaps it in. A failed reload must leave the previous snapshot active.
- Ensure routing/decision logging can include `model_registry_version` from the snapshot used for that decision.

## Files / Packages

Expected product-code areas for the implementing agent:

- `internal/registry` or equivalent package for registry types, snapshot storage, validation, lookup, and reload.
- `internal/router` integration points that read the active snapshot and log `model_registry_version`.
- Config or seed files for the MVP registry source if the repo already has a config location.
- Package-local tests for snapshot immutability, lookup, filtering, and reload behavior.

Do not add provider adapter logic in this issue; adapter dispatch only consumes registry data.

## Acceptance Criteria

- The service can load a model registry during startup and fail readiness if no valid active snapshot exists.
- The active registry snapshot is read-only from request-path code.
- Lookup by internal model id returns model metadata including provider id, provider model id, tier, capabilities, cost metadata, context window, enabled state, and latency/quality fields if configured.
- Lookup by provider id returns provider metadata or the provider models needed for dispatch.
- Capability filtering helpers return only enabled models that satisfy all requested capabilities.
- Reload validates and swaps the entire snapshot atomically.
- Failed reload keeps the previous `registry_version` active and reports a clear error.
- Router decisions can attach the snapshot `model_registry_version` used for the decision.
- No per-request database read or external lookup is introduced for registry access.

## Tests / Verification

- Add table-driven unit tests for model id lookup, missing model lookup, provider lookup, and provider model id mapping.
- Add table-driven unit tests for capability filters, including models missing one required capability and disabled models.
- Add tests proving reload success swaps `registry_version` and reload failure preserves the old snapshot.
- Add a test or code-level guard that snapshot readers cannot mutate shared registry state.
- Run focused package tests for the registry and any touched router integration packages.

## Out of Scope

- Dynamic provider health scoring.
- Tenant-specific allowlists.
- Admin HTTP endpoint design beyond an internal reload hook if needed.
- Automatic price refresh from provider APIs.
- Routing policy decisions or fallback construction.
- Provider adapter implementation.

## Dependencies

- Architecture source: `01-architecture/06-model-registry.md`.
- Routing and latency constraints from `AGENTS.md` and low-latency architecture docs.
- ISSUE-010 may consume the registry/profile IDs, but this registry package should stand on its own.

## Subagent Notes

- Keep this implementation deterministic and fast-path safe.
- Preserve the distinction between internal model ids and provider model ids.
- Prefer small Go packages and standard-library concurrency primitives such as `atomic.Value` or a carefully scoped `RWMutex`.
- If the registry source format is not already established, choose the smallest MVP format and document the assumption in the issue closeout.

## Klar när

- Issue spec has been implemented without introducing per-request registry I/O.
- Registry snapshot lookup/reload/filter tests pass.
- Startup/readiness behavior reflects whether a valid registry snapshot is loaded.
- Routing logs or decision metadata include the active `model_registry_version`.

## Closeout 2026-05-19

- Implementerat `internal/registry` med immutable snapshots, atomic reload via `Store`, deterministic capability filtering och lookups by model, provider och provider model id.
- Default MVP registry finns i `registry.DefaultDefinition()` och håller statisk metadata separat från framtida health overlays.
- Verifierat med `go test ./internal/registry -race -count=1` och full `go test ./... -race -count=1`.
