# Bandit routing (learn from outcomes)

Bandit routing lets the router **learn which model actually delivers for each
task class** from realized outcomes, instead of trusting static priors forever.
It is a UCB1 multi-armed bandit over `(task class, model)` pairs that balances
**exploiting** the best-known model against **exploring** under-sampled ones.

Fas 4 roadmap deliverables: *bandit-routing* and *lära routing från outcome-data*.

## Deterministic, on the fast path

UCB1's exploration bonus is a closed-form function of the pull counts — there is
**no randomness**. Given the same recorded stats, routing is fully reproducible,
so the bandit respects the project's deterministic-fast-path rule. Rewards are
recorded off the response path; scoring only reads in-memory counters.

## How it scores

For a `(task, model)` with `n` pulls, mean reward `μ`, and `N` total pulls for
the task:

```
score = μ + c · sqrt( ln(N) / n )     (clamped to [0,1])
```

- `μ` is the exploitation term — how well the model has actually done.
- the bonus is the exploration term — large when a model is under-sampled,
  shrinking as it is pulled, so promising-but-unproven models get tried and
  consistently-good models get exploited.
- `c` (`ROUTER_BANDIT_EXPLORATION`, default ≈1.41 = √2) tunes how eagerly it
  explores.

The score is exposed through the same `engine.QualitySource` interface as
eval-driven scoring (#52), so it plugs straight into the routing quality term. A
model with no pulls for a task returns "unknown" and scoring falls back to the
static/measured prior — the bandit only speaks once it has data.

## Reward signal

Today the reward is **attempt success** (1.0 for a successful provider call, 0.0
for an error/timeout), recorded per `(task, model)` on every attempt (primary,
fallback, and each council member). Richer outcome signals (human acceptance,
eval pass, cost-per-success) can feed the same `Record` path later without
changing the routing side.

## Warm start

When `ROUTER_MEASURED_QUALITY_REPORT` is set, the bandit **seeds** each arm from
the report's eval pass/fail counts, so it starts from the offline frontier
instead of cold and refines online. When bandit routing is on it supersedes the
static measured-quality source (which it subsumes).

## Configuration

```bash
ROUTER_BANDIT_ENABLED=true
ROUTER_BANDIT_EXPLORATION=1.41        # UCB1 c; higher = explore more
# optional warm start from offline evals:
ROUTER_MEASURED_QUALITY_REPORT=eval-report/report.json
```

Off by default. When off, routing uses static priors (or the measured report if
configured) exactly as before.

## Rollout

- Start with the default exploration and a warm start from the eval report so the
  bandit never routes blind.
- Watch per-arm stats (`Bandit.Snapshot()` — task, model, pulls, mean reward);
  a dashboard panel is a natural follow-up.
- Because scoring still weighs cost, latency, capability, and health, the bandit
  only shifts the *quality* term — it can prefer a proven-cheaper model but can't
  override a hard capability or health filter.
