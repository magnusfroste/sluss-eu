# 2026-05-28 - Policy Test Runner Strict YAML

## Context

A code review of the ISSUE-024 policy test runner found that YAML struct decoding accepted unknown fields. A typo such as `expect` or `matched_rule` could drop assertions and produce a false passing policy test.

## What I Learned

Policy test fixtures are executable safety checks, so permissive YAML decoding is unsafe. Runner input must reject unknown keys and empty expectation blocks before converting raw YAML into typed test cases.

## Reuse Rules

- Open this note when adding or changing policy test-runner fixture fields.
- Validate YAML keys at the node level before struct decoding when multiple top-level shapes are supported.
- Require every policy test case to contain at least one expected assertion.

## Failure Signals

- A policy test passes even though an `expected` key is misspelled.
- A test case has `expected: {}` or omits `expected` entirely.
- New fixture fields are added to structs but not to strict field allowlists.

## Next Checklist

- [ ] Update the strict field allowlist for each newly supported YAML field.
- [ ] Add a negative parser test for misspelled keys near the new field.
- [ ] Add or keep coverage that rejects empty expected assertions.
