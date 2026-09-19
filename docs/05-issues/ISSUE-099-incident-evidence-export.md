# ISSUE-099 — Incident-evidens: 24/72h-export (NIS2 rapporteringsfönster)

- category: enhancement
- state: done
- epic: EPIC-12
- priority: P1
- type: backend|product
- sprint: 09
- klar: 2026-07-12

## Bakgrund

NIS2 ger en väsentlig entitet 24 timmar för tidig varning och 72 timmar för
incidentanmälan. När något händer behöver CISO:n LLM-trafikens evidens för
fönstret NU — inte efter en logg-grävning. (Uppföljning på ISSUE-095.)

## Levererat

- **`GET /router/incident-report?window=24h|72h|7d`** (admin; `&format=json`):
  markdown-evidenspaket över fönstret — blockerade requests per kod /
  dataklassning / task-typ, känsliga prompts per klass och vart de faktiskt
  gick (lokal vs moln, molnläckor namnger modellen), aktiv policyversion,
  regim-inramning. Endast klassningar och antal — aldrig promptinnehåll;
  "evidence for a report, not a legal attestation".
- **MCP-verktyg `incident_report`** (window-arg) — agenten kan hämta samma
  evidens; returnerar både strukturerad JSON och markdown.
- **Durabel logg utökad**: `sensitivity` + `block_code` per request-rad i
  SQLite (idempotent migration), exponeras i request-loggposter; nytt
  fönsterquery `history.Since`.
- Evidenslänkar i policy-konsolen (control report · gap report · incident
  24h/72h · audit chain).

## Verifiering

Unit: fönsteraggregering (rader utanför fönstret räknas inte, molnläcka
namnger modell, okänt fönster → 72h), markdown/JSON-handler, 503 utan datadir,
MCP-verktyget. Live: konsolregel financial→block + riktig chattrafik → 403,
rapporten visar `console_rule_block`/financial/pv_console_001 korrekt.
