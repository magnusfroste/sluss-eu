# Council mode (Fusion-inspired deliberation)

For high-risk task classes a single model's answer is a single point of failure.
Council mode runs a **deliberation panel**: instead of routing to one model, the
router fans the request out to the top-N routed candidates **concurrently** and a
judge returns the answer the models agreed on. This trades cost and latency for
reliability — reserve it for the classes where a wrong answer is expensive
(`security_review`, `database_migration`, `unknown_high_risk`).

Inspired by OpenRouter Fusion (multi-model deliberation + judge). It realizes the
"premium **+ verifier**" tier from the task→tier table: the panel *is* the
cross-check.

## How it works

1. **Gate** — the request's task class is in `ROUTER_COUNCIL_TASKS` and it is
   **non-streaming** (the judge needs whole answers). Otherwise the normal
   single-model path runs, untouched.
2. **Panel** — built from the routing decision's selected model plus its
   fallback chain, keeping distinct `(provider, model)` pairs up to
   `ROUTER_COUNCIL_SIZE` (default 3). A panel of fewer than two distinct models
   falls back to the single-model path.
3. **Fan-out** — every member is called concurrently. Each call is recorded as an
   attempt, so health tracking and spend account for the **full** panel cost.
4. **Judge** — `ConsensusJudge` (deterministic, no LLM) normalizes each answer and
   clusters them by token-set (Jaccard) similarity. The answer the largest set of
   models agreed on wins; ties break toward the most-preferred (top-ranked)
   member. With no agreement it returns the top-ranked model's answer and flags
   the lack of consensus. Keeping the judge model-free preserves the project's
   "no model in the meta-decision" rule.

The winner's response is returned unchanged (OpenAI-compatible). The panel is
**slow path only** — fast-path routing never fans out and never calls a model to
decide.

## Response headers

| Header | Meaning |
|---|---|
| `X-Router-Council` | `true` when the panel ran |
| `X-Router-Council-Size` | number of panel members called |
| `X-Router-Council-Success` | members that returned a usable answer |
| `X-Router-Council-Agreement` | fraction of successful members that agreed with the winner (`0.00`–`1.00`) |
| `X-Router-Council-Reason` | e.g. *unanimous*, *majority consensus*, *no consensus* |
| `X-Router-Selected-Model` | the winning model (may differ from the primary routed model) |

A low `X-Router-Council-Agreement` is a signal: the models disagreed on a
high-risk task, and the answer deserves human review.

## Configuration

```bash
# Comma-separated task classes that deliberate (empty = off).
ROUTER_COUNCIL_TASKS=security_review,database_migration,unknown_high_risk
# Max distinct models in a panel (drawn from the routed fallback chain).
ROUTER_COUNCIL_SIZE=3
```

## Cost

A council of N runs N provider calls, so it costs ~N× a single call. That is the
point — buy reliability where it matters — but keep `ROUTER_COUNCIL_TASKS`
narrow. Every member is attributed in spend, so the dashboard reflects the true
cost honestly.
