# Autopilot (guardrailed policy tuning)

Autopilot closes the Fas 4 loop — **measure → recommend → apply** — but keeps a
human in the safety envelope rather than the per-change decision. It takes the
policy-floor recommendations mined from the eval frontier (`docs/... policy
recommendations`, ISSUE §11), runs them through **hard guardrails**, and emits an
auto-approved policy containing only the changes it is confident are safe. Every
other recommendation is rejected with a reason, on the record.

Fas 4 roadmap deliverable: *autopilot med guardrails*.

## Safety model

Autopilot is an **offline governance step** — it never touches the request path
and never writes the live policy. It produces artifacts:

- `autopilot-policy.yaml` — only the guardrail-approved changes, as a valid
  policy snippet.
- `autopilot-decisions.txt` — every recommendation and whether it was applied,
  rejected, or skipped, with the reason (the audit trail).

Adopting the policy is an explicit operator action (copy into your policy file +
reload — the router already supports policy reload without a deploy). A bad
change can therefore never silently reach production; autopilot only shrinks the
operator's job to review-and-adopt or reject.

## Guardrails

A recommended change is auto-approved only if it clears **all** of:

| Guardrail | Default | Rationale |
|---|---|---|
| Protected task classes never changed | `security_review`, `database_migration`, `unknown_high_risk` | a wrong downgrade on the premium+verifier tier is the most expensive mistake |
| Minimum eval samples | 20 | don't act on thin evidence |
| Downgrade pass-rate floor | ≥ 0.95 absolute | a cheaper tier must be *clearly* good enough |
| Margin above the recommendation target | ≥ 0.05 | require headroom, not a coin-flip pass |
| Blast-radius cap | 3 changes/run | limit how much can move at once; the biggest cost savings win the slots |

Upgrades (raising a floor) are safety-positive, so they only need enough
samples, not the downgrade pass-rate floor. `keep` / `no_model_meets_target` /
`insufficient_data` recommendations are skipped (no change to make).

## Usage

```bash
make autopilot
# or:
go run ./cmd/eval-report -autopilot
```

On the current dataset autopilot approves **0 changes** — the one non-`keep`
recommendation (`simple_code_edit`, 8 samples) is rejected for too few samples.
That is the point: autopilot does nothing unless the evidence is strong, so its
default posture is safe.

## Adopting an approved change

```bash
make autopilot
cat eval-report/autopilot-decisions.txt          # review what/why
# merge the rules you accept from eval-report/autopilot-policy.yaml into your
# policy file, then reload the router's policy (no deploy needed).
```
