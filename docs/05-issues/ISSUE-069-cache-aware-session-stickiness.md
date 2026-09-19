# ISSUE-069: Cache-aware routing — session stickiness

## Labels
- `epic: EPIC-11`
- `priority: P1`
- `type: enhancement`
- `sprint: post-beta`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Gör routningen cache-medveten: förfrågningar i samma konversation/session ska som huvudregel routas till **samma modell** som föregående tur, så att providerns prompt-cache träffar. Routing per prompt får inte i praktiken radera cache-besparingen.

## Bakgrund

Prompt caching är en av de största besparingsspakarna på API-nivå (cache-läsningar kostar ~1/10 av ordinarie input-pris hos Anthropic; oberoende produktionsanalys mäter −71,5 % för väl strukturerad caching, mot −77,1 % för modellroutning). Men cachen är **per modell och prefix**: en router som byter modell mitt i en konversation kastar bort providerns cache för hela historiken — routingens vinst kan ätas upp av förlorade cache-läsningar.

OpenRouters Auto Router löser exakt detta med *session stickiness* (modellvalet pinnas per session). Tokenizer saknar motsvarighet i dag: varje request klassificeras och routas oberoende.

## Acceptanskriterier

- Förfrågningar som tillhör samma konversation (heuristik: samma tenant + meddelandehistorikens prefix matchar en tidigare sedd konversation, alternativt klient-hintad sessionsnyckel i metadata) routas till samma modell som senast, även om klassificeringen denna tur pekar på en *billigare* modell.
- **Stickiness bryts uppåt, aldrig nedåt i skydd:** en tydlig eskalering (högre risk/task-klass, t.ex. summarization → security_review) får byta modell uppåt; en oskyldigare tur mitt i en session byter inte nedåt (cache + kvalitetskontinuitet vinner över marginalbesparingen).
- Stickiness respekterar policy: allow/deny, budget-nedgradering och conservative mode vinner alltid över stickiness.
- Beslutet syns i `decision_reasons` (t.ex. `"sticky: same session as req_x, cache-aware"`) och i request-loggen.
- TTL/gräns: stickiness gäller inom ett konfigurerbart fönster (t.ex. `ROUTER_STICKINESS_TTL_SECONDS`, default ~15 min) och nollställs när fönstret löper ut.
- Mätbarhet: dashboarden kan visa andel sticky-beslut, så effekten på cache-träffar går att följa upp.

## Tekniska noter

- Fast path-budgeten gäller: sessionsuppslag måste vara in-memory (jfr `decisioncache`) — ingen extern lagring i request-vägen.
- Konversationsidentitet utan att lagra prompttext: hash av normaliserat historik-prefix (jfr secret-masking-principen — inga råa prompts persisteras).
- Interagerar med `decisioncache` (ISSUE-052): stickiness är per-konversation, decision cache är per-identisk-request — dokumentera prioritetsordningen.
- Interagerar med eval-driven scoring (#52): sticky-beslut ska inte förorena outcome-/kvalitetsdata utan flaggas som sticky.

## Klar när

- Acceptanskriterierna uppfyllda, tester för stickiness-heuristiken och eskaleringsundantaget.
- Regression: `make smoke` grönt; streaming-fallet (usage-chunk, #51) fungerar oförändrat.
- `.env.example` + `deploy/example.env` dokumenterar TTL-variabeln.

## Härkomst

Identifierad i strategisk research (produktionsanalys av cache-vs-routing-samspelet; OpenRouter Auto:s session stickiness). Del av EPIC-11:s "dirigent"-tes: orkestrera plattformarnas inbyggda spakar (caching) i stället för att motarbeta dem.
