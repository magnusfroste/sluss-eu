# ISSUE-049: Skapa spend simulator

## Labels
- `epic: EPIC-10`
- `priority: P1`
- `type: data`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa spend simulator som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Simulerar baseline premium.
- Visar besparing.
- Visar riskjusterad besparing.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-10)

- `spend.Simulator` (i `internal/spend/simulator.go`) återanvänder `cost`-modellen
  (fixed-point micro-USD) och prissätter varje request mot en **premium-baseline**.
- `Run([]SimRequest)` returnerar `SimResult` med:
  - **Baseline premium** (vad allt skulle kostat på premium-modellen),
  - **Besparing** (baseline − faktisk routad kostnad) + procent,
  - **Riskjusterad besparing** — besparingen viktas per risknivå (`DefaultRiskWeights`:
    low=1.0, medium=0.85, high=0.5, critical=0.2; okänd → 0.5), eftersom aggressiv
    nedgradering av risktunga tasks bär kvalitets-/säkerhetsrisk.
- `SimResult.Summary(w)` renderar en läsbar rapport. `cost.FormatMicroUSD`
  exponerad för USD-formattering.
- Tester täcker baseline/besparing, riskviktning (känt värde), default-vikt för
  okänd risk, fel vid saknad cost-metadata och rendering. Ren beräkning — utanför
  fast path.
