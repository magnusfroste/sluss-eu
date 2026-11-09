# Request lifecycle

## Lifecycle för en request

```mermaid
sequenceDiagram
    participant C as Client
    participant P as Proxy
    participant A as Auth
    participant F as Feature Extractor
    participant R as Router
    participant E as Provider Executor
    participant M as Model Provider
    participant L as Event Log

    C->>P: POST /v1/chat/completions
    P->>A: validate api key
    A-->>P: tenant + policy context
    P->>F: extract features
    F-->>P: JobDescriptor
    P->>R: decide(JobDescriptor)
    R-->>P: RouteDecision
    P->>L: log decision
    P->>E: execute primary model
    E->>M: provider request
    M-->>E: response / stream
    E-->>P: normalized response
    P->>L: log attempt + usage
    P-->>C: response
```

## Steg

### 1. Ingress

- Validera JSON.
- Läs headers.
- Extrahera API key.
- Sätt request id.
- Starta tracing span.

### 2. Auth

- Identifiera tenant.
- Kontrollera key status.
- Läs budget och policyversion.
- Sätt begränsningar för model allowlist.

### 3. Feature extraction

Fast path ska vara deterministisk:

- Tokenestimat.
- Prompttyp.
- Riskord.
- Kodsignal.
- Context-size.
- Metadata.
- Explicit override.

### 3b. Lokal andra åsikt (valfri, av som standard)

Med `ROUTER_SECOND_OPINION_MODEL` satt frågas en lokal modell vilka dataklasser texten har. Den får bara lägga till en klass och höja risken, aldrig sänka. Timeout (standard 500 ms): därefter gäller reglernas resultat ensamt. Utan inställningen finns inget steg här, och beslutet är rent regelbaserat.

### 4. Policy evaluation

Policy kan:

- Blockera request.
- Tvinga modellklass.
- Kräva fallback.
- Kräva verifier.
- Sätta maxkostnad.
- Sätta retention.

### 5. Routing decision

Routern väljer:

- Primär modell.
- Fallbackmodell(er).
- Timeout per attempt.
- Max retries.
- Kontextstrategi.
- Beslutsförklaring.

### 5b. Mask and send

Har en matchad regel `mask: true` och svaret kommer från en modell som inte är lokal ersätts personuppgifter och nycklar i hela konversationen med platshållare innan något annat rör den (även före kontextpipelinen). Värdena finns bara i minnet för förfrågan. Auditposten `request.masked` har typer och antal.

### 6. Execution

Executor:

- Mappar OpenAI-format till providerformat.
- Skickar request.
- Streamar chunks vid behov.
- Normaliserar fel.
- Mäter tokens och latency.

### 7. Fallback

Fallback triggas vid:

- Timeout.
- Rate limit.
- Provider error.
- Policykrav.
- Verifier failure.

### 7b. Återställning och svarskontroll

Platshållare i svaret ersätts med värdena (även strömmat, där en platshållare kan delas mellan två bitar). Därefter läses svaret med samma värdedetektorer som frågan; nycklar och personuppgifter som frågan inte hade flaggas (`response.flagged`). Strömmade svar kontrolleras efter `[DONE]`, så inget fördröjs.

### 8. Response

Response ska inkludera standardfält plus optional routermetadata om klienten tillåter det.

Headers:

```text
x-router-request-id: req_...
x-router-selected-model: premium-reasoning
x-router-policy-version: pv_...
x-router-route-class: high_risk_code
```

## Latencykritiska punkter

- Auth ska cacha API key lookup.
- Policy ska vara precompiled.
- Modellregistry ska ligga i minne.
- Provider health ska läsas från minne (in-memory tracker).
- LLM-klassificering får inte användas på fast path.
