# Lightweight Classifier Experiment

This fixture set supports `ISSUE-058` as an offline experiment only.

## Guardrails

- The experiment lives outside the production routing path.
- No production rollout is allowed from this work alone.
- Any future rollout must go through a dedicated ADR and explicit wiring into the request path.

## Dataset

- Fixture: `docs/fixtures/classifier-experiment-dataset-v1.yaml`
- Train/test splits are committed in source to keep the experiment deterministic.
- Prompts use fabricated values only so the dataset can stay in-repo and under test.

## Verification

Run:

```bash
go test ./internal/classifier ./internal/evals -count=1
```
