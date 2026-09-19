# ISSUE-048: Skapa SDK metadata helpers

## Labels
- `epic: EPIC-10`
- `priority: P1`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera skapa sdk metadata helpers som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- SDK kan skicka project/task/risk.
- Exempel finns.
- Backward compatible.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-10)

- Nytt paket `internal/sdk` med en `Builder` som producerar routing-hints i två
  former: metadata-map (`Build`) för request-body och `X-Router-*`-headers
  (`Headers`). `Apply(req)` mergar in i `ChatRequest.Metadata` utan att skriva
  över befintliga nycklar.
- **SDK kan skicka project/task/risk** (+ tenant, sensitivity, latency, quality,
  budget, router-mode, requires_json_schema, samt `Set`-escape-hatch).
- Hint-typerna är **alias av routerns egna enums** → en sanningskälla. Ett
  round-trip-test kör hints genom `router.NewJobDescriptor` och verifierar att
  `ProjectIDHint`/`TaskTypeHint`/`RiskLevelHint` m.fl. landar — skyddar mot drift.
- **Backward compatible**: tom builder ändrar inget, `Apply` bevarar befintlig
  metadata, nil-säker.
- **Exempel**: körbart `examples/sdk-metadata/main.go` som visar båda formerna.
