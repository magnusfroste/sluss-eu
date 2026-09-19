# 2026-06-12 - Cost Quality Frontier Determinism

## Context

ISSUE-059 added an offline cost-quality frontier section to `cmd/eval-report` so eval artifacts can compare candidate models per task class without touching the routing fast path.

## What I Learned

The frontier score becomes unstable if it treats sparse eval pass-rate as the whole truth or if recommendation logic mixes outcome-style acceptance signals into the same number. A stable v1 shape is:

- keep frontier quality as a smoothed blend of routed eval passes plus registry quality priors
- keep outcome acceptance separate when it exists instead of folding it into the frontier score
- generate recommendations only from Pareto-frontier models with explicit deterministic tie-breaks

Using the registry prior as one pseudo-observation keeps zero-sample and one-sample task/model combinations from swinging too hard while still letting repeated eval wins dominate over time.

## Reuse Rules

- Open this note when changing offline eval reporting, model frontier summaries, or recommendation heuristics derived from eval artifacts.
- Keep the frontier report out of the request fast path; it should stay an offline analysis step.
- Blend sparse eval evidence with registry priors instead of replacing priors outright.
- Treat acceptance/outcome signals as separate fields unless there is an explicit product decision to redefine the frontier score.
- Break recommendation ties deterministically by the documented ordering, not by map iteration or float noise.

## Failure Signals

- A model with one passing routed case jumps to the top of every task class regardless of registry priors.
- Frontiers disappear or churn between identical runs because tied models are emitted in a different order.
- Outcome acceptance silently changes the frontier score even though the report claims to be eval-plus-prior only.
- A more expensive model with lower blended quality still appears as a frontier recommendation.

## Next Checklist

- [ ] Preserve the eval-plus-prior blend unless a new versioned frontier formula is introduced deliberately.
- [ ] Quantize or integerize cost fields before comparing or serializing recommendation candidates.
- [ ] Add explicit empty, single-model, and tie-case tests when frontier selection rules change.
