# Cost och budgeting

## Kostnadsestimat

Kostnad beräknas med:

```text
estimated_cost = input_tokens_estimate * input_price + max_output_tokens_estimate * output_price
```

## Budgetnivåer

- Tenant budget.
- Project budget.
- API key budget.
- Request max cost.
- Daily/monthly caps.

## Budgetpolicy

När budget nära gräns:

- Varna.
- Nedgradera låg-risk tasks.
- Blockera triviala requests om policy säger det.
- Fortsätt high-risk tasks om admin tillåter.

## Kostnadsrapport

Visa:

- Total spend.
- Spend per modell.
- Spend per provider.
- Spend per taskklass.
- Besparing mot baseline.
- Over-routing cost.
- Under-routing incidents.

## Baseline

För att beräkna besparing:

```text
baseline = kostnad om alla requests hade gått till premium-default
actual = faktisk routekostnad
savings = baseline - actual
```

## Cost per successful task

```text
cost_per_successful_task = total_cost / successful_outcomes
```

Detta är viktigare än ren kostnad per request.
