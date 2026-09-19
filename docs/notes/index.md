# Learning Notes Index

This index is a retrieval map for reusable project lessons. Open only notes relevant to the current task.

## High-Signal Recurring Lessons

- 2026-05-19 - [Streaming Adapter Compatibility](2026-05-19-streaming-adapter-compatibility.md): open when changing provider interfaces, streaming adapters, SSE framing, first-token fallback boundaries, or per-attempt stream cancellation.
- 2026-05-19 - [Immutable Registry Snapshots](2026-05-19-immutable-registry-snapshots.md): open when changing registry/profile snapshot fields, lookup helpers, capability filters, or health overlays.
- 2026-05-19 - [Context Processor Timeout Isolation](2026-05-19-context-processor-timeout-isolation.md): open when adding mutable request processors, nested request cloning, timeout handling, or fail-open pipeline behavior.
- 2026-05-19 - [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md): open when mapping client metadata/headers into routing, policy, classifier, or descriptor logging fields.
- 2026-05-28 - [Compiled Policy Cache Semantics](2026-05-28-compiled-policy-cache-semantics.md): open when changing policy compilation, route hint mapping, cache reload, default merging, or constraint aggregation.
- 2026-05-28 - [Policy Test Runner Strict YAML](2026-05-28-policy-test-runner-strict-yaml.md): open when adding policy test fixture fields or changing runner YAML decoding.
- 2026-06-12 - [Decision Comparison Determinism](2026-06-12-decision-comparison-determinism.md): open when comparing route decisions across policies, adding shadow-routing diffs, persisting decision comparisons on events, or stabilizing cost-delta reporting.
- 2026-06-12 - [Cost Quality Frontier Determinism](2026-06-12-cost-quality-frontier-determinism.md): open when changing offline cost-quality reports, blending eval pass-rate with registry priors, or altering frontier recommendation tie-breaks.
- 2026-05-19 - [Backlog Triage State Sync](2026-05-19-backlog-triage-state-sync.md): open when issue labels, `.ai/tasks.json`, closeout sections, and next-task selection might disagree.
- 2026-05-19 - [Classifier Keyword Boundaries](2026-05-19-classifier-keyword-boundaries.md): open when adding or widening classifier keyword rules for task, risk, sensitivity, or feature extraction.
- 2026-06-12 - [Runtime Policy Wiring Operability](2026-06-12-runtime-policy-wiring-operability.md): open when adding policy-gated runtime features, server config fields, or dashboard/eventlog trackers.
- 2026-06-12 - [Offline Classifier Experiment Boundary](2026-06-12-offline-classifier-experiment-boundary.md): open when adding trained classifier experiments or considering classifier runtime rollout.
- 2026-06-12 - [System Prompt Adapter Boundaries](2026-06-12-system-prompt-adapter-boundaries.md): open when adding prompt adapters, rewriting system-role messages, or introducing pre-provider request mutators.
- 2026-06-12 - [RBAC Role Compatibility Boundaries](2026-06-12-rbac-role-compatibility-boundaries.md): open when adding role checks, protecting control-plane routes, or rolling out explicit roles without breaking legacy API keys.

## Routing

- 2026-05-19 - [Immutable Registry Snapshots](2026-05-19-immutable-registry-snapshots.md): immutable in-memory registry snapshots; open when adding model metadata, provider lookups, provider-model mappings, capability filters, or health overlays.
- 2026-05-19 - [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md): trusted auth context vs untrusted client hints; open when building or consuming `JobDescriptor`.
- 2026-06-12 - [Decision Comparison Determinism](2026-06-12-decision-comparison-determinism.md): shared decision diff shape; open when adding shadow decisions, A/B policy simulation, event persistence, or route-change reporting.
- 2026-06-12 - [Cost Quality Frontier Determinism](2026-06-12-cost-quality-frontier-determinism.md): offline frontier blending and tie-break rules; open when changing task-class model recommendations or cost/quality summaries.

## Policy

- 2026-05-28 - [Compiled Policy Cache Semantics](2026-05-28-compiled-policy-cache-semantics.md): compiled policy snapshots, tenant/project cache lookup, route hint normalization, reload safety, default fill semantics, constraint merge semantics.
- 2026-05-19 - [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md): client `risk_level`, `task_type`, and sensitivity metadata stay hints until classifier/policy truth is applied.
- 2026-06-12 - [Runtime Policy Wiring Operability](2026-06-12-runtime-policy-wiring-operability.md): runtime policy file loading, server bootstrap config, operator enablement, and eventlog/dashboard fan-out.

## Providers

- 2026-05-19 - [Streaming Adapter Compatibility](2026-05-19-streaming-adapter-compatibility.md): optional streaming provider interface; open when adding adapters, stream support, unsupported-stream behavior, or timeout fallback cancellation.
- 2026-06-12 - [System Prompt Adapter Boundaries](2026-06-12-system-prompt-adapter-boundaries.md): existing `system` messages are the adapter surface; open when adding model-specific prompt rewrites or provider-side prompt shaping.

## Auth

- 2026-06-12 - [RBAC Role Compatibility Boundaries](2026-06-12-rbac-role-compatibility-boundaries.md): role checks stay separate from scopes; open when gating dashboard/control-plane routes, adding admin-only surfaces, or deciding how legacy unset roles should behave.

## Testing

- 2026-05-28 - [Policy Test Runner Strict YAML](2026-05-28-policy-test-runner-strict-yaml.md): strict YAML fixture decoding; open when adding policy test fields, parser formats, or expected assertions.
- 2026-05-19 - [Classifier Keyword Boundaries](2026-05-19-classifier-keyword-boundaries.md): keyword boundary regression coverage; open when classifier terms can overlap product/package names.
- 2026-06-12 - [Offline Classifier Experiment Boundary](2026-06-12-offline-classifier-experiment-boundary.md): fixed train/test split, rule baseline comparison, offline-only trained classifier tests.
- 2026-06-12 - [Decision Comparison Determinism](2026-06-12-decision-comparison-determinism.md): deterministic A/B and shadow diff coverage; open when deciding which route fields should count as a behavioral change or when dashboards consume comparison payloads.
- 2026-06-12 - [Cost Quality Frontier Determinism](2026-06-12-cost-quality-frontier-determinism.md): eval-plus-prior smoothing and frontier edge cases; open when adding model frontier fixtures or changing recommendation selection.
- 2026-06-12 - [System Prompt Adapter Boundaries](2026-06-12-system-prompt-adapter-boundaries.md): adapter tests should prove disabled-by-default, exact-target mutation, profile-target mutation, and message-order preservation.

## Operations

- 2026-05-19 - [Backlog Triage State Sync](2026-05-19-backlog-triage-state-sync.md): sync `05-issues/` triage labels and implementation closeouts with canonical `.ai` state on `main`; open when `continue` would pick the wrong task.
- 2026-06-12 - [Runtime Policy Wiring Operability](2026-06-12-runtime-policy-wiring-operability.md): verify shipped binary wiring when a feature passes tests only through injected package config.

## Performance

- 2026-05-28 - [Compiled Policy Cache Semantics](2026-05-28-compiled-policy-cache-semantics.md): validate and compile before atomic cache swap; fast-path evaluation should do only in-memory matcher checks.
- 2026-05-19 - [Classifier Keyword Boundaries](2026-05-19-classifier-keyword-boundaries.md): deterministic in-memory keyword matching; prefer bounded scans and explicit terms over broad prompt-derived outputs.
- 2026-06-12 - [Decision Comparison Determinism](2026-06-12-decision-comparison-determinism.md): micro-USD diff aggregation and route-field-only comparisons keep reporting deterministic without touching the fast path.
- 2026-06-12 - [Cost Quality Frontier Determinism](2026-06-12-cost-quality-frontier-determinism.md): keep frontier scoring offline, smoothed, and tie-stable when comparing models per task class.
- 2026-05-19 - [Streaming Adapter Compatibility](2026-05-19-streaming-adapter-compatibility.md): first emitted stream chunk is the fallback boundary; cancel abandoned attempts on pre-token fallback and do not retry after data is flushed.
- 2026-05-19 - [Immutable Registry Snapshots](2026-05-19-immutable-registry-snapshots.md): clone map/slice metadata on snapshot ingress and egress while keeping lookups deterministic and in-memory.
- 2026-05-19 - [Context Processor Timeout Isolation](2026-05-19-context-processor-timeout-isolation.md): hard timeouts do not stop goroutines; deeply clone mutable request state and commit only on successful completion.
- 2026-06-12 - [System Prompt Adapter Boundaries](2026-06-12-system-prompt-adapter-boundaries.md): mutate cloned system messages only after a match so disabled and default paths stay fast and unchanged.
- 2026-05-19 - [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md): descriptor construction stays in-memory and avoids logging raw metadata or prompt/message text.

## Security

- 2026-05-19 - [Classifier Keyword Boundaries](2026-05-19-classifier-keyword-boundaries.md): avoid false auth/secret/security sensitivity from substring matches like `token` inside `tokenizer`.
- 2026-05-19 - [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md): safe descriptor logs should expose derived fields and counts, not raw metadata values, metadata key names, prompt text, or message content.
- 2026-06-12 - [Offline Classifier Experiment Boundary](2026-06-12-offline-classifier-experiment-boundary.md): trained classifiers must stay offline until an ADR approves production request-path use.

## Chronological Notes

| Date | Title | Retrieval cues |
| --- | --- | --- |
| 2026-05-19 | [Streaming Adapter Compatibility](2026-05-19-streaming-adapter-compatibility.md) | provider, streaming, SSE, optional interface, first token boundary, cancellation |
| 2026-05-19 | [Immutable Registry Snapshots](2026-05-19-immutable-registry-snapshots.md) | registry, snapshot, map clone, slice clone, capability filter, health overlay |
| 2026-05-19 | [Backlog Triage State Sync](2026-05-19-backlog-triage-state-sync.md) | issue state, triage labels, implementation closeouts, .ai/tasks.json, next task drift |
| 2026-05-19 | [Context Processor Timeout Isolation](2026-05-19-context-processor-timeout-isolation.md) | contextproc, hard timeout, goroutine, race, nested metadata/tools clone |
| 2026-05-19 | [Job Descriptor Trust Boundaries](2026-05-19-job-descriptor-trust-boundaries.md) | JobDescriptor, trusted auth, untrusted metadata hints, risk hint, safe logging |
| 2026-05-19 | [Classifier Keyword Boundaries](2026-05-19-classifier-keyword-boundaries.md) | classifier, keywords, word boundary, false positive, sensitivity hints |
| 2026-05-28 | [Compiled Policy Cache Semantics](2026-05-28-compiled-policy-cache-semantics.md) | policy compiler, cache reload, route hints, default fill, constraint merge, tenant project scope |
| 2026-05-28 | [Policy Test Runner Strict YAML](2026-05-28-policy-test-runner-strict-yaml.md) | policy test runner, strict YAML, unknown fields, empty expected, false pass |
| 2026-06-12 | [Runtime Policy Wiring Operability](2026-06-12-runtime-policy-wiring-operability.md) | runtime policy, cmd/router, server config, eventlog fan-out, operator enablement |
| 2026-06-12 | [Offline Classifier Experiment Boundary](2026-06-12-offline-classifier-experiment-boundary.md) | trained classifier, offline experiment, ADR-gated rollout, baseline comparison |
| 2026-06-12 | [Decision Comparison Determinism](2026-06-12-decision-comparison-determinism.md) | decision diff, shadow routing, A/B policy simulation, event persistence, micro-usd, explanation churn |
| 2026-06-12 | [Cost Quality Frontier Determinism](2026-06-12-cost-quality-frontier-determinism.md) | frontier report, eval prior smoothing, pareto, recommendation ties, task-class cost quality |
| 2026-06-12 | [System Prompt Adapter Boundaries](2026-06-12-system-prompt-adapter-boundaries.md) | prompt adapter, system role, clone commit, model match, profile match |
| 2026-06-12 | [RBAC Role Compatibility Boundaries](2026-06-12-rbac-role-compatibility-boundaries.md) | rbac, auth, admin role, legacy key, dashboard gating, insufficient_role |
