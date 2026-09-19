# ISSUE-107 — Deterministisk och namngiven savings-baseline

- **Epic:** EPIC-15 (CISO dashboard)
- **Priority:** P1
- **State:** done (2026-07-19)

## Bakgrund (live-bugg)

Dashboardens "saved vs all-premium" visade **−254 %** i produktion. Rotorsak:
`premiumPricing` tog *första* premium-modellen i iterationsordningen som
baseline. Med två premium-modeller med 4× prisskillnad (glm-4.6 $0.60/$2.20
vs openai/gpt-4o $2.50/$10.00) blev baselinen glm medan trafiken till stor
del gick till gpt-4o → "du betalade mer än allt-på-premium". En siffra som
ska övertyga en CISO får inte bero på map-ordning.

## Ändring

- `premiumPricing` väljer nu **deterministiskt den dyraste** aktiverade
  premium-chat-modellen (högst summerat in+ut-pris) — den realistiska
  "vad du hade betalat utan routing"-counterfactualen — och returnerar även
  modellens namn (`ProviderModelID`, fallback `ID`).
- Namnet flödar genom `server.Config.PremiumBaselineModel` →
  `DashboardOptions` → `SavingsSummary.BaselineModel` (JSON
  `baseline_model`) och visas i widgeten: *"routing everything to the
  premium model (openai/gpt-4o) would have cost …"* — transparens om vilken
  counterfactual som prissätts.

## Tester

- `cmd/router/premium_baseline_test.go` — två premium-modeller med olika
  pris: dyraste väljs, oavsett ordning.
- `dashboard_savings_test.go` — baseline-namnet följer med i summeringen.

## Not

Baselinen läses vid boot (som tier-/prisändringar i övrigt — "changes apply
on restart"). Ägar-runbook för negativa savings: kontrollera att tier-nivåerna
speglar prisordningen (en "balanced"-modell dyrare än premium ger tunn
savings med ärlig matte).
