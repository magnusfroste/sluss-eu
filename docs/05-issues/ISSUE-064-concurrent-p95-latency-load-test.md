# ISSUE-064: Concurrent p95 latency load test

## Labels
- `epic: EPIC-10`
- `priority: P2`
- `type: test`
- `sprint: 08`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Lägg till ett repeterbart concurrent-lasttest som mäter routerns p95-latens under samtidig last, så beta-gatens latency-punkt kan bekräftas med evidens snarare än sekventiella enstaka anrop.

## Bakgrund

Under lokal beta-validering mot OpenRouter mättes latensen end-to-end men endast **sekventiellt** (p50≈600 ms, p95≈780 ms inkl. providerns nät + inferens). Routerns egna overhead-mål (p95 < 100 ms före providern) verifieras separat av metrics, men beta-release-checklistans punkt "p95 under last" saknar ett concurrent-mätvärde. Detta issue stänger den luckan.

## Acceptanskriterier

- Ett lasttest (t.ex. `make load` eller ett script under `scripts/`) kör N samtidiga klienter mot `/v1/chat/completions` under en bestämd varaktighet.
- Rapporterar p50/p95/p99 och felkvot; separerar gärna routerns overhead (X-Router-* / metrics) från total end-to-end-tid.
- Kör deterministiskt mot mock-providern i CI; en `-live`-variant mot en riktig provider när nyckel finns (jfr `make smoke` / `make smoke-live`).
- Resultatet kan kopplas till beta-release-checklistans latency-punkt.

## Tekniska noter

- Återanvänd mönstret från `scripts/smoke.sh` / `scripts/smoke-openrouter.sh` (bygg router, boota, kör, riv ner).
- Bevara fast path-latencybudgeten; lasttestet ska inte kräva ändringar i request-vägen.
- Håll det hermetiskt grönt i CI (mock); live-varianten skippar rent när nyckel saknas.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Testet passerar mot mock i CI.
- Beta-release-checklistan refererar mätvärdet.
