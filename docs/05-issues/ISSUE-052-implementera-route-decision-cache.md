# ISSUE-052: Implementera route decision cache

## Labels
- `epic: EPIC-05`
- `priority: P1`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera route decision cache som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Cache key versionerad.
- Endast låg-risk tasks cacheas.
- Policyversion invalidar.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-11)

Nytt paket `internal/decisioncache` (implementerar `06-engineering/07-caching.md`):

- **Cache key versionerad**: `Key` hashar (sha256) tenant/projekt/task/router-mode +
  prompt-fingerprint + metadata-fingerprint + **policy_version** + **registry_version**.
- **Endast låg-risk cacheas**: `Cacheable` kräver `RiskLow` + `SensitivityNone`;
  high-risk/sensitiva requests re-evalueras alltid (`X-Router-Cache: bypass`).
- **Policyversion invaliderar**: versionerna ingår i nyckeln → en policy-/registry-
  ändring ger ny nyckel, gamla beslut serveras aldrig.
- TTL-bunden in-memory-cache (`New(ttl, max)`); TTL≤0 inaktiverar. Chat-handlern
  slår upp före `Decide` och sätter `X-Router-Cache: hit|miss|bypass`. Cachen
  lagrar bara routing-beslutet — providern körs ändå.
- Wiring via `server.Config.DecisionCache` + `cmd/router/main.go`
  (`ROUTER_DECISION_CACHE_TTL_SECONDS`, default 60).
- Tester: nyckel-stabilitet/versionering, gating, Get/Put/TTL-expiry, disabled/nil-
  säkerhet, max-bound, samt engine-backat handler-test (miss→hit).
