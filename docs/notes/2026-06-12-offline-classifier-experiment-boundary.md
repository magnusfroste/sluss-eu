# 2026-06-12 - Offline Classifier Experiment Boundary

## Context

ISSUE-058 introduced a trained lightweight classifier experiment. The task explicitly required a dataset, baseline comparison against rules, and no production rollout without an ADR.

## What I Learned

Classifier experiments are safest when they consume existing feature extraction and rule outputs offline, but never replace `router.NewJobDescriptor` behavior. This keeps routing determinism, confidence semantics, conservative mode, and fast-path latency unchanged while still making model-vs-rules evidence collectable.

## Reuse Rules

- Put trained classifier experiments behind experiment-only APIs, fixtures, and tests.
- Compare against the current rule classifier using a fixed train/test split.
- Document production rollout as ADR-gated before adding any request-path switch.
- Include a fabricated-secret guard for committed prompt datasets.

## Failure Signals

- Experiment code imports or edits server/request-path routing.
- `router.NewJobDescriptor` starts selecting task or risk labels from trained output without an ADR.
- Dataset examples include realistic secret patterns or production-like customer text.
- Tests assert only trained-model metrics without checking the rule baseline.

## Next Checklist

- [ ] Confirm production classifier behavior is unchanged.
- [ ] Run focused classifier/evals tests.
- [ ] Record baseline and trained metrics from the fixed split.
- [ ] Require a new ADR before adding any runtime flag or rollout path.
