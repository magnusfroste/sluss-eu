# ISSUE-058: Implementera trained lightweight classifier experiment

## Labels
- `epic: EPIC-03`
- `priority: P2`
- `type: data`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera trained lightweight classifier experiment som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Dataset definierat.
- Baseline jämförs med rules.
- Ingen production rollout utan ADR.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- Offline-only deterministisk lightweight classifier experiment i `internal/classifier` med perceptron-liknande träning över prompttokens, extraherade features och baseline-output.
- Dataset/loader i `internal/evals` och `docs/fixtures/classifier-experiment-dataset-v1.yaml`, med train/test-split och secret-guard.
- Baseline jämförs mot tränad modell i tester; produktionens `router.NewJobDescriptor` och request path är oförändrade.
- Dokumentation anger att production rollout kräver separat ADR.
