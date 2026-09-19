# ISSUE-050: Integrera med CI för evals

## Labels
- `epic: EPIC-08`
- `priority: P1`
- `type: infra`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera integrera med ci för evals som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Eval smoke test körs i CI.
- Policyändringar testas.
- Rapport artifact skapas.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-10)

- **Eval smoke i CI**: nytt steg `make test-eval` i `.github/workflows/ci.yml`.
- **Policyändringar testas**: nytt steg `make test-policy` (golden-cases).
- **Rapport-artifact**: nytt `cmd/eval-report` kör datasetet via eval-runnern och
  skriver `report.json` + `report.txt` (`evals.FormatReport`). CI-steget
  `make eval-report` genererar dem och `actions/upload-artifact@v4` laddar upp
  `eval-report/` (med `if: always()` + `if-no-files-found: error`).
- `cmd/eval-report` har flaggor `-dataset`, `-out`, `-min-pass` (kan agera gate).
  `eval-report/` är gitignorad.
- Tester: `generate` kör datasetet (≥50 fall), `writeReport` producerar giltig
  JSON + icke-tom text, fel vid saknat dataset.
