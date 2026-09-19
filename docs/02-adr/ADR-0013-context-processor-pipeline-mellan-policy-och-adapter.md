# ADR-0013: Context-processor-pipeline mellan policy och adapter

## Status

Accepterad (2026-05-19)

## Kontext

Modellval är största ROI-hävstången men context är största absoluta token-volymen i agent-flöden. Tokenizer behöver en framtida väg att addera context-trimning (RTK-liknande filtering, dedup, redaction, summarization) utan att röra routing-koden. Designen måste skydda fast-path-budget p95 < 100 ms (ADR-0004) och får inte sänka klient-förtroendet.

## Beslut

En ordnad pipeline av `ContextProcessor`-implementationer körs **mellan policy-engine och provider-adapter**, på den provider-neutrala `NormalizedModelRequest`.

1. **Position:** efter policy, före adapter-translation. Processorer arbetar bara med normaliserat format.
2. **Åtkomst:** muterbar `*NormalizedModelRequest`, read-only `*JobDescriptor`. Processor kan välja att avstå baserat på `task_type`/`risk_level`.
3. **Inget escalering tillbaka till routing.** Pipeline är straight-line, fail-open. En processor får logga risk-signal men ändrar inte fattat route-beslut. Eliminerar loops.
4. **Latensbudget:** total pipeline 20 ms hård cap, per-processor 10 ms soft / 15 ms hård. Vid timeout: skippa processor, fortsätt med opåverkad request.
5. **Opt-in:** per-tenant via policy YAML. Inte via klient-header (klienter ljuger; policy är server-controlled). Per-task_type kan uttryckas i policy senare.
6. **Observability:** response-header `X-Router-Context-Savings: <tokens>` ackumulerat över pipelinen, samt event-log fält `context_processors_applied: [{name, tokens_saved}]`.
7. **Default off.** Tenant måste explicit aktivera. Vi modifierar klientens input — högt förtroende krävs.

V1 levererar enbart interfacet, no-op-pipelinen och feature-flaggan (`ROUTER_CONTEXT_PIPELINE_ENABLED`). Streaming-requests hoppar över pipelinen tills explicit stöd byggs.

## Konsekvenser

- Tokenizer kan addera context-besparingar inkrementellt utan arkitekturändring.
- Latensbudget förblir skyddad via hård cap och fail-open-semantik.
- Klient-förtroende skyddat via default-off och per-tenant opt-in.
- Streaming täcks inte i v1 — separat designfråga senare.

## Alternativ

- Köra processors i adaptern (provider-specifikt format) — förkastat: varje processor skulle behöva känna varje provider.
- Tillåta processors att escalera till routing — förkastat: introducerar loops, bryter fast-path-bestämbarhet.
- Per-request opt-in via header — förkastat: klient-headers är otillförlitliga som säkerhetskontroll.
