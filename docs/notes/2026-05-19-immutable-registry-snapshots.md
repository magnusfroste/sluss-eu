# 2026-05-19 - Immutable Registry Snapshots

## Context

ISSUE-009 introduced in-memory model registry snapshots for the routing fast path. Snapshot readers need cheap lookups, but returned model metadata includes slices and maps such as strengths and quality scores.

## What I Learned

Returning a struct by value is not enough to make snapshot reads immutable when the struct contains reference types. Maps and slices must be cloned at snapshot build time and again on read boundaries, otherwise tests or request-path code can mutate shared registry state.

## Reuse Rules

- Keep snapshot maps private and expose lookup methods instead of raw maps.
- Return copied model/profile values when they contain slices or maps.
- Sort ids before returning filtered model lists so fast-path behavior stays deterministic.
- Keep dynamic health state out of static snapshot structs; layer it separately later.

## Failure Signals

- A test mutates a returned model's `QualityScores` or `Strengths` and later reads the mutated value from the snapshot.
- Capability filters return models in inconsistent order between runs.
- Static registry structs start accumulating mutable health, timeout, or error-rate fields.

## Next Checklist

- When adding registry fields, check whether they are reference types and clone them on ingress and egress.
- When adding lookup helpers, return deterministic ordering for slices.
- When adding health-aware routing, keep health in a separate overlay keyed by internal model id.
