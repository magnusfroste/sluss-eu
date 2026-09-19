# ISSUE-019: Mät feature extraction latency

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: test`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Lägg till benchmark/performance guard för feature extraction så fast path bevarar dokumenterat p95-mål under 20 ms och inte råkar introducera LLM, nätverk, DB eller tung tokenizer i classifier-kedjan.

## Implementation Contract

- Benchmark ska mäta den lokala feature extraction-kedjan från request/messages till extractor-output som används av `JobDescriptor`.
- Mät cases som täcker kort prompt, medium kodprompt, stor kodprompt, stack trace, SQL migration, auth/payment/security keywords och tool/json-schema request.
- Benchmarkdata ska vara deterministisk och incheckad som lokala fixtures eller inline testdata.
- Ingen benchmark eller performance guard får göra nätverksanrop, DB-anrop, LLM-anrop, provider lookup eller live policy fetch.
- Performance guard ska rapportera p95 eller en approximativ p95 från upprepade lokala runs på ett stabilt sätt. Om Go benchmarkformen inte passar p95 direkt, lägg en separat unit/perf test som mäter durations över fixtures.
- Guarden ska faila när feature extraction p95 överskrider 20 ms på dokumenterad lokal fixtureprofil, eller åtminstone exponera en tydlig test failure/threshold i CI-kompatibel form.
- Benchmark ska inte vara beroende av maskinspecifik wall-clock för hårda mikrosekunder på ett bräckligt sätt. Använd rimliga sample sizes och stabila fixtures så 20 ms-målet fångar grova regressioner.
- Mät inte provider latency eller total router overhead i detta issue.

## Files / Packages

- Förväntad produktkod/testkod: benchmark och performance test nära feature extractor-paketet, till exempel `internal/classifier/features`.
- Förväntad dokumentation i testnamn/kommentar: referens till 20 ms p95 från `01-architecture/05-low-latency-architecture.md` och `06-engineering/06-latency-budget.md`.
- Förväntad CLI-verifiering: fokuserad `go test`/`go test -bench` för extractorpaketet.
- Håll produktändringen till benchmark/performance guard och eventuell minimal testbar instrumentation.

## Acceptance Criteria

- Feature extraction benchmark finns och kan köras lokalt utan externa beroenden.
- Performance guard täcker samma signaltyper som ISSUE-016: code blocks, stack traces, file paths, SQL/migration, auth/payment/security keywords och tool/json-schema.
- p95-resultat rapporteras i test output eller benchmark helper output.
- Guarden har dokumenterad threshold på `< 20 ms` p95 för feature extraction.
- Testdata är deterministic och fri från secrets/PII.
- Benchmarkkedjan verifierar att inga network/LLM calls används genom package design eller test double/guard där det är praktiskt.

## Tests / Verification

- Kör fokuserad benchmark för feature extractor-paketet.
- Kör performance guard som failar på threshold breach.
- Unit/perf test: alla fixturetyper körs igenom extractor utan panics och utan rå promptlogging.
- CI-kompatibel kommandoform dokumenteras i testfil eller issue-closeout, till exempel `go test ./internal/classifier/... -run TestFeatureExtractionLatency`.

## Out of Scope

- Ingen optimering av provider dispatch, policy evaluation eller model scoring.
- Ingen total router p95-mätning.
- Ingen produktkod utöver eventuell testbar instrumentation som redan behövs för extractor.
- Ingen live load test mot server.

## Dependencies

- ISSUE-016 för feature extractor som ska mätas.
- ISSUE-015 om tokenestimat ingår i extractor-kedjan.
- `01-architecture/05-low-latency-architecture.md`
- `06-engineering/06-latency-budget.md`

## Subagent Notes

- Bygg testdata lokalt och deterministiskt. Undvik fixtures som kräver repo-scan eller läser stora externa filer.
- Skriv thresholden så den skyddar mot grova regressioner utan att bli flakig på utvecklarmaskiner.
- Om extractor inte finns ännu, skapa benchmarkstrukturen i samma package när ISSUE-016 landar.

## Klar när

- Feature extraction har benchmark och CI-kompatibel performance guard mot 20 ms p95.
- Benchmarkdata är lokal, deterministic och täcker featurefamiljerna från ISSUE-016.
- Acceptance criteria och fokuserad verifiering passerar.
