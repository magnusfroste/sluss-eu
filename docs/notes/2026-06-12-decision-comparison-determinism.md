# 2026-06-12 - Decision Comparison Determinism

## Context

ISSUE-054 added offline A/B policy simulation so the same eval dataset can be routed under two different compiled policies and compared without any provider execution. ISSUE-055 reused the same comparison contract in the live request path for shadow routing, where production executes only the primary provider call while persisting actual-vs-shadow policy diffs for dashboard/API consumers.

## What I Learned

A reusable decision-diff contract becomes noisy fast if it treats metadata-only churn as a routing change. The stable contract for offline simulation and live shadow routing should compare route-relevant fields only, quantize cost deltas to micro-USD, and keep policy-version drift visible but separate from the behavioral "decision changed" count.

When shadow routing is added to the request path, the alternate decision must stay computation-only. Persist the shared `DecisionComparison` on the decision event and feed dashboards from that payload instead of re-running or re-shaping comparisons downstream.

## Reuse Rules

- Open this note when adding shadow routing, offline policy comparison, or decision-diff reporting.
- Compute the shadow decision with the same engine/policy inputs but never execute a shadow provider call.
- Keep the shared comparison shape anchored on route-relevant fields: selected model/provider, blocked state, fallback chain, timeout, verifier requirement, and estimated cost.
- Treat `policy_version` as informative metadata, not by itself a changed decision.
- Quantize cost deltas before comparison or aggregation so JSON/text reports stay deterministic across runs.
- Ignore explanation-text differences in change detection; explanations are useful context but unstable as a behavioral diff key.
- Persist and fan out the shared comparison object directly on the decision event so logs, dashboards, and JSON APIs all consume the same contract.

## Failure Signals

- A case is reported as "changed" only because the compared policies have different version strings.
- Cost deltas flap between zero and tiny floating-point differences across identical runs.
- Shadow-routing or eval reports churn because explanation strings changed while the selected route stayed the same.
- Aggregated cost deltas do not equal the sum of per-case deltas.
- A shadow-routing implementation accidentally triggers a second provider call or stream attempt.

## Next Checklist

- [ ] Reuse the same comparison shape for shadow-routing persistence/reporting instead of inventing a second diff format.
- [ ] Keep shadow execution on the decision side only; the primary adapter path should still execute exactly once.
- [ ] Preserve micro-USD aggregation when adding dashboards or JSON consumers.
- [ ] Add focused tests whenever new route-decision fields should or should not influence change detection.
