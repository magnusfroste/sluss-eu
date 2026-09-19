# ISSUE-059: Implementera cost-quality frontier report

## Labels
- `epic: EPIC-08`
- `priority: P2`
- `type: data`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera cost-quality frontier report som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Rapport visar modeller per taskklass.
- Cost vs quality visualiseras.
- Rekommendationer genereras.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- Eval reports innehåller nu cost-quality frontier per taskklass med modellrader, snittkostnad och quality v1.
- Quality v1 blandar eval pass-rate med registry quality priors; outcomes hålls separat och vägs inte in i frontier-poäng.
- Pareto-frontier/dominated-markering och deterministiska rekommendationer för `best_value`, `lowest_cost`, `highest_quality`.
- `report.json` och `report.txt` innehåller frontier-sektion; tester täcker tomma/single/tie-fall.
