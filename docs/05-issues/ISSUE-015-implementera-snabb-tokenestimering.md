# ISSUE-015: Implementera snabb tokenestimering

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: backend`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Implementera en deterministic, billig tokenestimator för fast path så `JobDescriptor.prompt_tokens_estimate` och `JobDescriptor.max_output_tokens_estimate` kan sättas utan tung tokenizer, DB, nätverk eller LLM.

## Implementation Contract

- Använd char-baserad approximation enligt low-latency-arkitekturen: `estimated_tokens = ceil(char_count / 4)`.
- Räkna tecken över alla relevanta chat messages som skulle skickas till provider: system, developer, user, assistant och tool messages om de finns i requestkontraktet.
- Message aggregation ska vara deterministisk och inte kräva att messages sammanfogas till en stor promptsträng om streaming/JSON-formen redan finns. Summera längder per message/content-part där det går.
- Hantera multimodal eller icke-textuell content konservativt: textdelar räknas via chars/4, icke-textdelar får en dokumenterad placeholder/overhead om de stöds, annars ignoreras inte tyst utan markeras som feature för senare routing.
- `max_output_tokens_estimate` ska sättas från explicit `max_tokens`/`max_completion_tokens` när klienten anger det. Om klienten saknar outputlimit ska en konservativ default användas enligt packagekonstant/policydefault och markeras som estimate.
- Estimatorn får returnera både promptestimat, outputestimat och enkel metadata som char count och om värdet var client-specified eller defaulted.
- Estimatorn ska kunna användas av ISSUE-014 när `JobDescriptor` skapas.
- Håll implementationen allocation- och latency-medveten. Den ska passa under feature extraction-budgeten och inte introducera tung tokenizer på fast path.

## Files / Packages

- Förväntad produktkod: intern classifier/feature package, till exempel `internal/classifier` eller `internal/router/features`.
- Förväntad integration: descriptor builder från ISSUE-014 fyller `prompt_tokens_estimate` och `max_output_tokens_estimate`.
- Förväntade tester/benchmarks: package-nära unit tests och microbenchmark för estimatorn.
- Håll produktändringen till estimator, descriptor-integration och fokuserade tester.

## Acceptance Criteria

- `ceil(chars/4)` används exakt för textbaserade promptestimat.
- Flera messages aggregeras korrekt utan att bara räkna sista user message.
- Tomma/missing messages ger `0` promptestimat och inga panics.
- Explicit outputlimit prioriteras för `max_output_tokens_estimate`.
- Saknad outputlimit ger en konservativ default som är dokumenterad i kod/test.
- Estimatet skrivs till `JobDescriptor` utan att prompttext sparas.
- Latencybenchmark finns med förväntan att estimatorn är långt under feature extraction p95-budgeten på 20 ms för lokala testfall.

## Tests / Verification

- Unit test: 1, 4, 5 och 8 tecken ger ceil-beteende enligt `ceil(chars/4)`.
- Unit test: system + user + assistant/tool content summeras.
- Unit test: `max_tokens` och `max_completion_tokens` hanteras enligt OpenAI-kompatibel requestform.
- Unit test: saknad outputlimit använder default och markerar estimate/default source.
- Benchmark: estimatorn körs på kort, medium och stor lokal promptdata utan nätverk/LLM.
- Kör fokuserade Go-tester och benchmark för estimatorpaketet.

## Out of Scope

- Ingen exakt modelltokenizer.
- Ingen async re-tokenization pipeline.
- Ingen kostnadsberäkning eller budgetpolicy.
- Ingen provider-specifik tokenmodell.

## Dependencies

- ISSUE-014 för `JobDescriptor`.
- `01-architecture/05-low-latency-architecture.md`
- `06-engineering/02-job-descriptor-schema.md`
- `06-engineering/06-latency-budget.md`

## Subagent Notes

- Gör estimatorn ren och testbar: input request/messages in, estimat ut.
- Bevara fast path-regeln: ingen DB, inget nätverk, ingen LLM, ingen tung tokenizer.
- Samordna fältnamnen med ISSUE-014 så outputen kan kopplas utan adapterglapp.

## Klar när

- Estimatorn är implementerad, kopplad till descriptorn och täckt av unit tests.
- Benchmark visar lokal latens med god marginal under feature extraction-budgeten.
- Acceptance criteria och fokuserade tester passerar.
