# ISSUE-010: Definiera interna modellprofiler

## Labels
- `epic: EPIC-02`
- `priority: P0`
- `type: product`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Define internal model profiles and tier mapping so policy and routing logic can request stable internal capabilities without depending on provider marketing names or raw provider model ids.

The first profile set must include the tiers `cheap`, `balanced`, and `premium`, plus named internal profiles used by policy such as profile ids for reasoning, coding, cheap/simple work, and balanced default routing. Provider model mapping must stay behind registry/profile ids.

## Implementation Contract

- Add internal model profile definitions that policy can reference by stable profile id.
- Define the base tiers `cheap`, `balanced`, and `premium` as product/routing concepts, not direct provider model ids.
- Each profile must resolve to one or more registry model ids, not directly to provider model ids.
- Provider model ids remain owned by the registry and provider adapter layer.
- Profiles may include required capabilities, intended task classes, tier preference, and optional fallback profile/model references if already supported by the router design.
- Policy/routing code should consume profile ids or tier/profile selectors, then resolve through the registry snapshot for actual model/provider selection.
- Make profile resolution deterministic and fast-path safe. No DB, HTTP, LLM, or provider calls during per-request profile resolution.
- Ensure logs and decision explanations preserve internal profile/model identity and can still include the selected provider after registry resolution.

## Files / Packages

Expected product-code areas for the implementing agent:

- `internal/registry`, `internal/profiles`, or equivalent package for profile/tier definitions and validation.
- `internal/policy` or router policy integration points that reference profile ids instead of provider model ids.
- Existing registry config/seed data if profiles are stored with model metadata.
- Package-local tests for profile validation and resolution.

Do not create or update real provider adapters in this issue.

## Acceptance Criteria

- `cheap`, `balanced`, and `premium` tiers are explicitly represented and documented in code/config.
- Named internal profiles exist for policy use and are not provider marketing names.
- Profile resolution maps profile id/tier to registry model ids, then uses registry metadata to find provider/provider model ids.
- Policy examples or tests demonstrate policy selecting an internal profile instead of a provider model id.
- Resolution rejects missing profiles, profiles with no enabled registry models, and profiles whose target models lack required capabilities.
- Decision metadata can expose selected internal profile/model id while keeping provider-specific ids behind the registry/provider boundary.
- Fast path remains in-memory and deterministic.

## Tests / Verification

- Add table-driven unit tests for tier/profile lookup: `cheap`, `balanced`, `premium`, missing profile, and disabled target model.
- Add tests showing provider model ids are not required in policy inputs.
- Add tests for profile capability constraints, for example `tool_calls` or `json_schema` required but no eligible model available.
- Add a focused policy/router test that resolves a profile to a registry model id.
- Run focused package tests for profile, registry, and any touched policy/router packages.

## Out of Scope

- Provider adapter request/response mapping.
- Dynamic health-aware model ranking.
- Tenant-specific model allowlists.
- Admin UI or profile management endpoints.
- Automatic quality-score learning from outcomes.

## Dependencies

- Architecture source: `01-architecture/06-model-registry.md`, especially the rule to avoid provider marketing names in policy.
- ISSUE-009 registry snapshot APIs if implemented first.
- Policy/routing issue work that consumes internal profile ids.

## Subagent Notes

- Keep profile ids boring, stable, and internal. They should survive provider/model swaps.
- If the implementing agent must choose where profile config lives, prefer colocating it with registry config unless the repo already has a clearer policy config surface.
- Do not let policy import provider adapter constants or raw provider model names.

## Klar när

- Internal profiles and tiers are available to policy/routing code.
- Provider model ids remain hidden behind registry/profile resolution.
- Profile resolution tests pass.
- The issue closeout names any selected profile ids and where they are defined.

## Closeout 2026-05-19

- Implementerat `internal/profiles` med tiers `cheap`, `balanced`, `premium` och profilerna `cheap-general`, `balanced-coder`, `premium-reasoning`.
- Profile resolution mappar endast till interna registry model ids; provider model ids ligger bakom registry metadata.
- Verifierat missing profile, disabled target model och required capability failures med `go test ./internal/profiles -race -count=1`.
