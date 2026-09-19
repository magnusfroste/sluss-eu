# ISSUE-014: Skapa JobDescriptor datamodell

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: backend`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Skapa `JobDescriptor` som routerns interna kontrakt mellan ingress, feature extraction, policy och routing. Kontraktet ska normalisera OpenAI-kompatibla chat completion requests till en snabb, loggningssäker och policybar representation utan att läcka prompttext.

## Implementation Contract

- Definiera en intern `JobDescriptor` med fält enligt `06-engineering/02-job-descriptor-schema.md`: request/tenant/project-id, `task_type`, `risk_level`, `sensitivity`, tokenestimat, capability flags, preferenser, `files_touched`, `keywords`, `router_mode`, `explicit_model` och `metadata`.
- Lås vokabulären för risk till `low`, `medium`, `high`, `critical`.
- Lås task-vokabulären till minst de klasser som används i routing/classifier-issues: `simple_chat`, `trivial_git`, `simple_shell`, `summarization`, `simple_code_edit`, `hard_code_debugging`, `security_review`, `database_migration`, `long_context_analysis`, `creative_copy`, `unknown_high_risk`.
- Mappa request metadata och headers till tenant context och descriptorfält utan att lita blint på klienten.
- Header-/metadata-mappning ska kunna läsa tenant/project/router hints, latency/quality/budget preferences, explicit model/router mode och SDK-signaler, men dessa ska behandlas som untrusted hints som policy senare kan ignorera, blockera eller eskalera.
- Prompttext får användas för feature extraction och tokenestimat men får inte lagras i descriptorn eller ingå i strukturerad decision logging.
- Logging av descriptor får bara innehålla säkra härledda signaler som tokenestimat, booleans, task/risk, filnamn/pathar och keyword-listor. Logga inte råa messages, prompttext, secrets eller PII.
- Descriptorn ska vara billig att skapa på fast path: inga DB-anrop, inga nätverksanrop, ingen LLM och ingen tung tokenizer.

## Files / Packages

- Förväntad produktkod: intern request/classifier/routing-yta, till exempel `internal/router`, `internal/classifier` eller befintligt närliggande paket om repo redan har en tydligare struktur.
- Förväntade tester: package-nära unit tests för descriptor-konstruktion och loggsäker serialisering.
- Håll ändringen till descriptor-kontrakt, byggare och fokuserade tester.

## Acceptance Criteria

- `JobDescriptor` finns som internt Go-kontrakt och kan byggas från en OpenAI-kompatibel chat completion request.
- Tenant context mappas från verifierad auth-/API-key-kontext i första hand och från headers/metadata endast som hints.
- `metadata` behåller klientens extra hints utan att göra dem policy-sanning.
- Rå prompttext/messages serialiseras inte i descriptor logging.
- `files_touched` och `keywords` representerar härledda signaler och inte rå promptdump.
- Risk-, sensitivity- och taskfält använder samma vokabulär som ISSUE-017 och ISSUE-018.

## Tests / Verification

- Unit test: metadata/header hints mappas till descriptorfält och tenant context enligt kontraktet.
- Unit test: klientmetadata kan ange `risk_level: low`, men descriptor/policy-yta markerar signalen som hint och tillåter senare eskalering.
- Unit test: descriptor/log payload innehåller inte rå `messages[].content`.
- Unit test: defaultvärden sätts deterministiskt när metadata saknas.
- Kör fokuserade Go-tester för paketet som äger descriptorn.

## Out of Scope

- Ingen routing-score, provider selection eller fallbackkedja.
- Ingen persistent datamodell eller DB-migration.
- Ingen fullständig policy engine.
- Ingen LLM-baserad klassificering.

## Dependencies

- `06-engineering/02-job-descriptor-schema.md`
- `06-engineering/03-classifier-design.md`
- `01-architecture/04-routing-engine.md`
- `01-architecture/05-low-latency-architecture.md`
- ISSUE-015 för tokenestimat.
- ISSUE-016 för `files_touched`, `keywords` och feature flags.

## Subagent Notes

- Håll detta som ett internt kontrakt. Undvik att exponera `JobDescriptor` som publik API-respons i detta issue.
- Var extra försiktig med loggtester: de ska visa frånvaro av prompttext, inte bara närvaro av descriptorfält.
- Samordna fältnamn med Worker C-issues så task/risk-vokabulären inte divergerar.

## Klar när

- Descriptor-kontraktet är implementerat, testat och används vid ingress till router/classifier.
- Säker serialisering/loggning bevisar att prompttext inte loggas.
- Acceptance criteria och fokuserade tester passerar.
