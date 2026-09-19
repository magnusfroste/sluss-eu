# ISSUE-013: Implementera cost estimator

## Labels
- `epic: EPIC-02`
- `priority: P0`
- `type: backend`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Implement a deterministic cost estimator that calculates estimated and actual request cost from registry cost metadata plus estimated or observed token counts.

The estimator must be safe for fast-path use, decimal-safe, and reusable by routing decisions, `/router/decision`, and post-response accounting.

## Implementation Contract

- Add a cost estimator package/function that accepts model registry cost metadata and token counts.
- Inputs must support estimated token counts before provider call and actual usage counts after provider response.
- Use registry cost fields such as `input_per_million` and `output_per_million`.
- Return structured output including input cost, output cost, total cost, currency if represented, model id, provider id if available, and whether the result is estimated or actual.
- Use deterministic decimal-safe calculation. Avoid float rounding drift for persisted/accounting values; represent money in fixed-point units, decimal strings, or integer micros/cents according to repo conventions.
- Handle missing cost metadata explicitly with a typed error or unavailable result, not silent zero cost.
- Handle zero-token, input-only, output-only, and large-token cases.
- Keep estimator pure and in-memory. It must not read DB, call provider APIs, or mutate registry state.
- Keep routing integration optional and small: router can use the estimator for `estimated_cost_usd`, but this issue is not a routing policy rewrite.

## Files / Packages

Expected product-code areas for the implementing agent:

- `internal/cost`, `internal/router/cost`, or equivalent estimator package.
- Registry model metadata types if cost fields need typed normalization.
- `/router/decision` or router decision output integration if already present and low-risk.
- Table-driven unit tests in the estimator package.

Do not create provider adapter code in this issue.

## Acceptance Criteria

- Estimator calculates input, output, and total cost from per-million token rates and token counts.
- Estimator supports both estimated pre-call counts and actual post-call usage counts.
- Calculation is deterministic and avoids binary floating-point money drift.
- Missing or invalid cost metadata is surfaced clearly.
- Zero and partial usage cases produce correct non-negative results.
- Large token counts do not overflow the chosen representation under realistic model limits.
- Router decision output can include `estimated_cost_usd` when registry cost metadata and estimated token counts are available.
- No external lookup is introduced for cost calculation.

## Tests / Verification

- Add table-driven unit tests for standard input/output cost, zero tokens, input-only, output-only, missing metadata, decimal rates, and large token counts.
- Add tests proving deterministic string/fixed-point output for rates like `1.0` input per million and `3.0` output per million.
- Add tests for estimated versus actual usage mode.
- Add a focused router decision test for `estimated_cost_usd` only if the decision endpoint already has a stable test seam.
- Run focused cost package tests and any touched router tests.

## Out of Scope

- Provider price refresh jobs.
- Tenant billing, invoices, or quota enforcement.
- Dynamic routing policy changes based on cost.
- Currency conversion.
- Tokenization-accurate prompt counting if the repo does not already provide it; this estimator consumes token counts supplied by another component.

## Dependencies

- Cost metadata from `01-architecture/06-model-registry.md`.
- API decision output shape from `01-architecture/10-api-contracts.md`.
- ISSUE-009 registry metadata if implemented first.

## Subagent Notes

- Prefer a tiny pure function with explicit inputs/outputs over hidden package globals.
- Keep units impossible to confuse: name fields with `_tokens`, `_per_million`, `_micros`, or `_usd` as appropriate.
- If the repo has no decimal/money convention, choose integer micro-USD or decimal string output and document it in closeout.

## Klar när

- Cost estimator accepts registry cost metadata plus estimated/actual token counts.
- Deterministic decimal-safe tests cover the main pricing cases.
- Missing metadata and edge cases are handled explicitly.
- Any router decision integration remains pure and in-memory.

## Closeout 2026-05-19

- Implementerat `internal/cost` med pure fixed-point micro-USD estimation from registry cost metadata plus estimated/actual token counts.
- Parser för USD-per-million rates undviker binary float money drift och returnerar structured input/output/total cost strings.
- Verifierat zero, partial, missing metadata, decimal rates, large token counts och estimated/actual modes med `go test ./internal/cost -race -count=1`.
