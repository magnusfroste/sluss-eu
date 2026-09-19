# Live policy A/B (experiments)

Sluss can run a **live A/B test** between two routing policies: a share of
real traffic is served an experiment policy variant while the rest stays on the
primary policy, and every request is tagged with its arm so outcomes can be
compared. This is the online counterpart to the offline policy simulation
(`eval-report -policy-a -policy-b`) and the runtime *shadow* comparison
(`ROUTER_SHADOW_POLICY_PATH`, which only observes) — here the variant is actually
served.

Fas 4 roadmap deliverable: *A/B-testing*.

## Assignment

Each request is bucketed by a stable hash of **tenant + project**:

- The same tenant/project always lands in the same arm (a stable experience,
  and clean per-arm outcome attribution) — across requests and restarts.
- An empty tenant/project is never bucketed into treatment.
- The split is a fast-path, in-memory hash — no I/O, no LLM.

`ROUTER_EXPERIMENT_PERCENTAGE` is the treatment share (0–100). `ROUTER_EXPERIMENT_SALT`
rotates the assignment: change it to reshuffle which tenants are in treatment
without changing the percentage (e.g. to start a fresh experiment).

## What the variant changes

Treatment requests are routed under the policy at `ROUTER_EXPERIMENT_POLICY_PATH`
instead of the primary policy. Everything else (feature extraction, scoring,
provider execution, fallback) is identical, so the experiment isolates the policy
as the single variable. The decision cache key already includes the policy
version, so the two arms never share cached decisions.

## Observability

| Signal | Where |
|---|---|
| `X-Router-Experiment-Arm` | response header: `control` or `treatment` (absent when no experiment runs) |
| `ExperimentArm` on the decision event | event log — slice spend, latency, and outcomes by arm |

Because the arm rides the existing decision event, per-arm cost/latency/outcome
analysis reuses the outcome store and event log already in place; a dedicated
per-arm dashboard panel is a natural follow-up.

## Configuration

```bash
ROUTER_EXPERIMENT_POLICY_PATH=policies/experiment.yaml
ROUTER_EXPERIMENT_PERCENTAGE=10   # 10% of tenants served the variant
ROUTER_EXPERIMENT_SALT=exp-2026-07
```

Empty path or `0%` disables the experiment (all control, no header) — the default.

## Safe rollout

- Off by default; when off there is zero change to the request path.
- Start small (a few percent), watch the per-arm outcome/cost signals, then widen
  `ROUTER_EXPERIMENT_PERCENTAGE` or promote the variant to the primary policy.
- The variant is a real compiled policy, validated at startup like any other; a
  bad file fails fast at boot rather than silently mis-routing.
