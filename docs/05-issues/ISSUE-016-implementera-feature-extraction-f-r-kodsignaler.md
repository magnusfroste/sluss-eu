# ISSUE-016: Implementera feature extraction för kodsignaler

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: backend`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Implementera deterministisk feature extraction för kod- och riskrelaterade promptsignaler så taskklassning, riskklassning och routing kan ske utan LLM eller externa anrop.

## Implementation Contract

- Feature extractor ska läsa requestens textinnehåll och säkra metadata och producera härledda signaler för `JobDescriptor`.
- Extrahera booleans/counts för kodblock, inline-kod och stack traces.
- Extrahera filpaths till `files_touched` när prompten nämner sannolika paths, till exempel `src/auth/session.ts`, `internal/router/foo.go`, `db/migrations/001_init.sql`, `package.json`.
- Detektera SQL/migration-signaler: `SELECT`, `ALTER TABLE`, `CREATE INDEX`, `.sql`, `migration`, `schema`, `rollback`.
- Detektera auth-signaler: `auth`, `oauth`, `session`, `jwt`, `token`, `password`, `permission`, `rbac`.
- Detektera payment-signaler: `payment`, `billing`, `checkout`, `stripe`, `invoice`, `subscription`, `refund`.
- Detektera security-signaler: `security`, `vulnerability`, `xss`, `csrf`, `ssrf`, `injection`, `secret`, `exploit`, `cve`.
- Detektera tool/json-schema requirements från request fields och prompt hints: tools/function calling, `response_format`, JSON schema, "return JSON", "structured output".
- Returnera extraherade keyword-listor som normaliserade, deduplicerade lower-case signaler. Begränsa liststorlek med konstant så logs inte blir promptdump.
- Feature output ska kunna mata `JobDescriptor.requires_code`, `requires_tool_use`, `requires_json_schema`, `files_touched`, `keywords`, `sensitivity` hints och task/risk-regler i ISSUE-017/018.
- Ingen rå prompttext ska lämna extractor-resultatet.

## Files / Packages

- Förväntad produktkod: intern feature/classifier package, till exempel `internal/classifier/features`.
- Förväntad integration: descriptor builder från ISSUE-014 och task/risk-regler från ISSUE-017/018 konsumerar extractor-output.
- Förväntade tester: table-driven unit tests med lokala promptsträngar och OpenAI-kompatibla requestfragment.
- Håll produktändringen till extractor-kontrakt, descriptor/classifier-integration och fokuserade tester.

## Acceptance Criteria

- Markdown-kodblock och inline-kod detekteras.
- Typiska Go/JS/Python stack traces detekteras utan LLM.
- Filpaths extraheras, dedupliceras och bevarar relevant case/pathform där det behövs för debugging.
- SQL/migration/auth/payment/security keywords extraheras till `keywords`.
- Tool och JSON-schema requirements sätts från både requeststruktur och tydliga promptsignaler.
- Extractor-output innehåller inte rå prompttext.
- Outputen är deterministisk för samma input och fri från nätverk/LLM.

## Tests / Verification

- Unit test: markdown-kodblock sätter `has_code_block`/`requires_code`.
- Unit test: stack trace med `panic`, `at ...`, `Traceback` eller Go stack frame sätter stacktrace-signal.
- Unit test: filpaths som `src/auth/session.ts` och `internal/router/route.go` extraheras.
- Unit test: migrations-/SQL-prompt ger `migration`, `sql` keywords och migration-signal.
- Unit test: auth/payment/security-promptar ger respektive keyword-listor.
- Unit test: `tools`/`response_format` och "return JSON matching schema" sätter tool/json-schema flags.
- Kör fokuserade Go-tester för featurepaketet.

## Out of Scope

- Ingen taskklassning eller riskklassning i detta issue, utöver att producera signalerna de behöver.
- Ingen modellrouting eller policy evaluation.
- Ingen exakt parser för alla programmeringsspråk.
- Ingen promptlogging eller storage av rå text.

## Dependencies

- ISSUE-014 för descriptorfält som ska fyllas.
- ISSUE-015 för char/tokencount-signaler.
- ISSUE-017 och ISSUE-018 som konsumenter av feature-output.
- `06-engineering/03-classifier-design.md`
- `01-architecture/05-low-latency-architecture.md`

## Subagent Notes

- Designa extractor-resultatet som ett litet struct-kontrakt som är lätt att testa och logga säkert.
- Håll keywordlistan som signaler, inte som promptsammanfattning.
- Var konservativ med false negatives för auth/payment/security: hellre en extra riskhint än att missa en känslig signal.

## Klar när

- Feature extractor producerar de avtalade kod-, path-, keyword-, tool- och JSON-schema-signalerna.
- Extractor-resultat kan användas av descriptor och classifier utan rå prompttext.
- Acceptance criteria och fokuserade tester passerar.
