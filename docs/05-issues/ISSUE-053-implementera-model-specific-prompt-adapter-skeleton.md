# ISSUE-053: Implementera model-specific prompt adapter skeleton

## Labels
- `epic: EPIC-06`
- `priority: P2`
- `type: backend`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera model-specific prompt adapter skeleton som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Adapters kan modifiera systemprompt.
- Disabled default.
- Test med två modellprofiler.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- Ny `internal/provider` prompt-adapter som är disabled-by-default och bara muterar befintliga `system`-rollmeddelanden i `NormalizedModelRequest.Messages`.
- Stöd för exact-model och profilbaserade prefix/suffix-regler med clone-then-commit så default/non-target-paths är oförändrade och meddelandeordning bevaras.
- Hook i chat-path efter context pipeline och före provider `Complete`, med strukturerad loggning av regelnamn utan promptinnehåll.
- `cmd/router` kan aktivera adaptern via `ROUTER_PROMPT_ADAPTER_ENABLED`; default är fortsatt disabled.
- Tester täcker disabled default, två modell/profil-matcher, clone-säkerhet och server-hook.
