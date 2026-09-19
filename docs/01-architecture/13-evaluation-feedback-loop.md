# Evals och feedback loop

## Varför evals behövs

Routing utan evals blir magkänsla. För att veta om routern sparar pengar utan att sänka kvalitet behövs testfall och outcome-data.

## Evaltyper

### Offline evals

Kör historiska eller syntetiska prompts mot flera modeller.

Mät:

- Korrekthet.
- Kostnad.
- Latency.
- Tokenanvändning.
- Verifierresultat.

### Shadow evals

Routern väljer modell A i produktion men simulerar vad den hade valt enligt policy B.

### A/B routing

En andel traffic får ny policy. Jämför outcome och kostnad.

### Real-world outcomes

Samla användarsignaler:

- Accepterad response.
- Rejection/regenerate.
- Test pass/fail.
- Manuell override.
- Code diff accepted.

## Eval dataset för MVP

Minst 50 testfall:

- 10 triviala git/shell.
- 10 enklare kodändringar.
- 10 summeringar.
- 10 debugging.
- 5 security review.
- 5 database/migration.

## Evalformat

```yaml
id: eval_001
name: trivial commit message
task_type: trivial_git
risk_level: low
prompt: |
  Write a concise commit message for this diff...
expected_route:
  tier: cheap
quality_check:
  type: llm_judge_or_rule
  criteria:
    - concise
    - mentions changed files
    - no fabricated changes
```

## Metrics

- Route accuracy mot expected route.
- Quality score.
- Cost per successful eval.
- Latency.
- Failure rate.
- Under-routing rate: billig modell valdes när premium behövdes.
- Over-routing rate: premium valdes när billig räckte.

## Feedback till routing

Initialt:

- Analysera manuellt.
- Uppdatera policy.
- Uppdatera quality scores.

Senare:

- Träna lightweight classifier.
- Multi-armed bandit per taskklass.
- Automatisk policyrekommendation.

## Guardrail

Automatisk optimering får inte sänka modellnivå för high-risk tasks utan explicit policyändring.
