# ISSUE-022: Implementera compiled policy cache

## Labels
- `epic: EPIC-04`
- `priority: P0`
- `type: backend`
- `sprint: 04`
- `category: enhancement`
- `state: done`

## Intent

Implementera en compiled policy cache som gör policyutvärdering billig på fast path. Parser/validator från ISSUE-021 får köra vid load/reload; request path ska bara läsa immutable, förkompilerad policydata från minne.

## Implementation Contract

- Skapa en cache som mappar tenant/project/policy-version till compiled policy snapshot.
- Compiled snapshot ska vara immutable efter publicering. Reload ska bygga en ny snapshot och byta atomiskt, inte mutera aktiv data under läsning.
- Fast path får inte göra DB-read, fil-read, nätverksanrop eller YAML/JSON-parse.
- Kompilering ska förbereda datastrukturer för snabb matchning: hashsets för allowed/denied providers/models/capabilities, ordnade regelgrupper, precompiled globmatchers och normalized string predicates.
- Cache-read ska returnera policyversion och compiled policy eller en tydlig default/fallback enligt repo-policy.
- Stale-policy behavior ska vara explicit: vid reloadfel ska senast giltiga policy fortsätta användas och felet loggas; ingen trasig policy får ersätta aktiv snapshot.
- Invalidation/reload ska kunna triggas explicit av load/admin/test-kod. Eventuell polling/worker är inte nödvändig i detta issue om repo saknar sådan infrastruktur.
- Concurrency ska vara säker för många readers och en reload writer. Använd standardbibliotekets synkroniseringsprimitiver och undvik lock som hålls under request-tungt arbete.
- Exponera minimal observability: current policy version, last reload result, last reload error och cache hit/miss counters om metricsytan finns.
- Inga raw promptfält ska lagras i cache eller logs.

## Files / Packages

- Förväntad produktkod: `internal/policy` för compiled types/cache, eventuellt integration i server/router bootstrap.
- Förväntad integration: request path kan hämta compiled policy från cache före policy evaluation.
- Förväntade tester: unit tests för compile, cache read, reload, stale fallback och concurrency.
- Håll ändringen till cache/compile-yta. Full policy evaluation kan vara minimal om den behövs för tests, men routing behavior hör till senare issues.

## Acceptance Criteria

- Valid parsed policy kan kompileras till immutable snapshot.
- Fast path cache lookup kräver inga DB-/fil-/parse-anrop.
- Reload med valid policy byter atomiskt till ny version.
- Reload med invalid policy behåller senaste giltiga version och returnerar/loggar fel.
- Concurrent readers under reload får antingen gammal eller ny giltig snapshot, aldrig halvmuterad data.
- Policyversion följer med snapshot och kan användas i route decision/logging.
- Default/fallback policy är explicit när tenant/project saknar policy.

## Tests / Verification

- Unit test: compile bygger matcher/hashsets och behåller rule order där det krävs.
- Unit test: cache lookup returnerar rätt tenant/project/version.
- Unit test: successful reload publicerar ny snapshot atomiskt.
- Unit test: failed reload behåller gammal snapshot.
- Unit test: `go test -race` på policypaketet eller focused concurrency test om feasible.
- Unit test: fast path lookup mockar storage och visar att storage inte anropas under read.
- Kör fokuserade Go-tester för policypaketet.

## Out of Scope

- Ingen full admin API för policy upload.
- Ingen persistent policy store om den inte redan finns.
- Ingen policy test runner.
- Ingen model scoring eller candidate filtering.

## Dependencies

- ISSUE-020 för DSL-semantik.
- ISSUE-021 för parsed/validated policystruktur.
- ISSUE-010 för profile/registry metadata.
- `01-architecture/05-low-latency-architecture.md`
- `01-architecture/07-policy-engine.md`

## Subagent Notes

- Prioritera immutable snapshots framför clever mutation. Det gör request-path-läsning enklare och säkrare.
- Var tydlig med fallback vid reloadfel; det är bättre att köra senast kända bra policy än att publicera halvtrasig policy.
- Håll cache API litet så routing och policy evaluation inte kopplas till storage.

## Klar när

- Compiled policy cache finns, är concurrency-safe och håller fast path fri från parse/storage.
- Reload och stale fallback är testade.
- Acceptance criteria och fokuserade tester passerar.
