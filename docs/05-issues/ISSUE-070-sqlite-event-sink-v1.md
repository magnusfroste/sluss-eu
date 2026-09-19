# ISSUE-070: SQLite event-sink — durabel historik för v1.0

## Labels
- `epic: EPIC-11`
- `priority: P1`
- `type: enhancement`
- `sprint: v1.0`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Ge single-container-deployen (v1.0) **durabel per-request-historik** utan externa beroenden: en SQLite-fil på den monterade volymen (`ROUTER_DATA_DIR`) som event-kön skriver till och dashboarden läser från. Ersätter Postgres-sinken som persistensspår för v1.0; Postgres + Redis är Enterprise 2.0 (se DECISION_LOG 2026-07-04).

## Bakgrund

I dag är request-loggen en in-memory-ring (senaste 100, nollställs vid omstart) och spend/savings en JSON-snapshot. För pilotkunder som ska **fatta köpbeslut i sin egen dashboard** behövs historiken över tid: fullständig request-logg (OpenRouter-klass arkiv), spend/savings-ackumulering och closed-loop-datan (ISSUE-065: retry-signaler, shadow-deltan, acceptans).

Arkitekturen passar SQLite perfekt (samma profil som systerprojektet agentanbud, 40 redeploys utan problem): event-kön är **en enda asynkron writer**, dashboarden är read-heavy, och en fil på volymen behåller "backup = cp"-enkelheten.

## Acceptanskriterier

- Ny eventlog-handler `sqlitesink` som persisterar decision- + attempt-events (samma join-logik som RequestLogTracker: decision skapar rad, attempt fyller tokens/kostnad) till `<ROUTER_DATA_DIR>/history.db`.
- **Ren Go:** `modernc.org/sqlite` (ingen CGO) — den statiska binären och `CGO_ENABLED=0`-bygget bevaras. WAL-mode + busy_timeout satta vid öppning.
- Skrivning sker **endast** i event-köns worker (aldrig i request-vägen); fel loggas och får aldrig påverka routing (samma kontrakt som övriga handlers).
- Dashboarden läser "Recent requests" och savings-aggregat från SQLite när sinken är aktiv (fallback till in-memory när `ROUTER_DATA_DIR` saknas) — historiken överlever omstart/redeploy.
- `spend.json`-snapshotten ersätts av (eller migreras in i) SQLite-aggregat; engångsmigrering vid start om filen finns.
- Retention: befintliga `ROUTER_RETENTION_DAYS`-sweepen städar gamla rader.
- Inga råa prompts eller secrets i databasen (samma fält som dagens RequestLogRecord; jfr secret-masking-principen).
- Tester: sink-enhetstester (join, WAL-samtidighet writer+läsare, retention), omstartstest (skriv → stäng → öppna → läs), och `make smoke` grönt.

## Tekniska noter

- Schema: `requests` (time, request_id, tenant, task, risk, model, provider_model_id, provider, in_tokens, out_tokens, cost_usd, blocked, sticky m.fl.) + små aggregat-vyer för dashboarden; index på (time), (tenant, time).
- Dashboard-frågor ska tåla 100k+ rader (LIMIT/index — SQLite klarar detta trivialt).
- Volymen finns redan (named volume `tokenizer-data:/data`, ägd av `app` — fixat i #56).
- Bygg vidare på, ersätt inte, `eventlog.MultiHandler`-fan-outen.

## Klar när

- Acceptanskriterierna uppfyllda; full svit grön; `make load` opåverkad (fast path rör inte SQLite).
- Deploy-checklistan uppdaterad: "Redeploy → historik + savings kvar".
- Dokumentation: deployment-topology-notis om v1.0-lagringsmodellen.

## Härkomst

Beslut 2026-07-04 (DECISION_LOG): v1.0 = en container + SQLite; Postgres/Redis = Enterprise 2.0. Konceptlån från systerprojektet agentanbud ("en container, en SQLite-fil, en cron-rad").
