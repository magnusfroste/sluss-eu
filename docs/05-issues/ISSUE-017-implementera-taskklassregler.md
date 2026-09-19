# ISSUE-017: Implementera taskklassregler

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: backend`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Implementera deterministiska taskklassregler som mappar feature-output till en initial `task_type` och confidence utan LLM-anrop på fast path.

## Implementation Contract

- Klassificeringen ska använda `JobDescriptor`/feature-output från ISSUE-014/016 och returnera `task_type`, `confidence` och `signals`.
- Stöd minst sex taskklasser från arkitekturdokumenten. Minsta obligatoriska vokabulär för detta issue:
  - `simple_chat`
  - `trivial_git`
  - `summarization`
  - `simple_code_edit`
  - `hard_code_debugging`
  - `security_review`
  - `database_migration`
  - `unknown_high_risk`
- Det är tillåtet att också stödja `simple_shell`, `long_context_analysis` och `creative_copy` om det passar befintlig struktur.
- Regler ska vara ordnade så mer specifika/högre risk-klasser vinner över breda generiska klasser.
- Confidence ska vara numerisk i intervallet `0.0` till `1.0` och byggas deterministiskt från matchade signaler.
- Returnera en signal-lista som förklarar beslutet, till exempel `stack_trace`, `code_block`, `auth_keyword`, `migration_keyword`, `short_prompt`, `diff_keyword`.
- Konservativ unknown behavior: låg confidence plus risk-/sensitivity-signaler ska ge `unknown_high_risk`, inte `simple_chat`.
- Låg confidence utan riskindikatorer får falla tillbaka till `simple_chat` eller annan balanserad default enligt packagekontrakt, men ska markeras med låg confidence.
- Inga nätverksanrop, DB-anrop eller LLM-anrop får ske i taskklassningen.

## Files / Packages

- Förväntad produktkod: intern classifier package, till exempel `internal/classifier`.
- Förväntad integration: descriptor builder sätter `task_type` och metadata/signals från classifier-resultatet.
- Förväntade tester: table-driven golden tests för varje taskklass och konfliktfall.
- Håll produktändringen till taskklassregler, descriptor-integration och fokuserade golden tests.

## Acceptance Criteria

- Minst de åtta obligatoriska taskklasserna ovan kan returneras.
- Stack trace + code/path ger `hard_code_debugging` med högre confidence än `simple_code_edit`.
- `commit message` + diff eller gitord ger `trivial_git`.
- `migration`, `.sql` eller SQL schemaändring ger `database_migration`.
- `security`, `vulnerability`, `xss`, `csrf`, `secret` eller liknande ger `security_review`.
- Kort prompt utan kod, tools eller risksignaler ger `simple_chat`.
- Sammanfatta/summary-signaler utan hög risk ger `summarization`.
- Riskindikatorer med låg task-confidence ger `unknown_high_risk`.
- Alla beslut returnerar confidence och signals.

## Tests / Verification

- Golden test: commit-message/diff prompt -> `trivial_git`.
- Golden test: kort allmän fråga -> `simple_chat`.
- Golden test: "summarize this log/doc" -> `summarization`.
- Golden test: liten kodändring utan stack trace -> `simple_code_edit`.
- Golden test: stack trace + auth path -> `hard_code_debugging`.
- Golden test: migration SQL -> `database_migration`.
- Golden test: XSS/secret/security review -> `security_review`.
- Golden test: okänd prompt med `production`, `secret` eller PII hint -> `unknown_high_risk`.
- Kör fokuserade Go-tester för classifierpaketet.

## Out of Scope

- Ingen risknivåimplementation utöver att lämna signals till ISSUE-018.
- Ingen model scoring, provider selection eller policy override.
- Ingen lightweight ML classifier i V1.
- Ingen LLM-classifier.

## Dependencies

- ISSUE-014 för `JobDescriptor`.
- ISSUE-016 för feature-output.
- ISSUE-018 för riskklassning på samma signals.
- `06-engineering/03-classifier-design.md`
- `01-architecture/04-routing-engine.md`

## Subagent Notes

- Håll reglerna läsbara och ordnade efter specificitet. Det ska vara uppenbart varför en golden case landar i sin klass.
- Samma signalnamn ska användas här och i riskreglerna för att undvika parallella vokabulär.
- Vid konflikt ska känsliga/riskabla klasser vinna över billigare taskklasser.

## Klar när

- Taskklassreglerna är implementerade, deterministiska och täckta av golden tests.
- Confidence och signals följer kontraktet.
- Acceptance criteria och fokuserade tester passerar.
