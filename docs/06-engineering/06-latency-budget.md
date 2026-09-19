# Latency budget

## Mål

Fast path routing ska lägga till mindre än 100 ms p95 före provideranrop.

## Budget

| Komponent | p95-budget |
|---|---:|
| Ingress + JSON parse | 5 ms |
| Auth cache | 10 ms |
| Feature extraction | 20 ms |
| Policy evaluation | 10 ms |
| Model scoring | 10 ms |
| Event enqueue | 5 ms |
| Provider dispatch overhead | 20 ms |
| Buffert | 30 ms |
| Total | 100 ms |

## Tekniska regler

- Ingen DB read i normal fast path.
- Ingen LLM-call i normal fast path.
- Inga externa prisuppslag per request.
- Inga synkrona dashboardaggregat.
- Inga tunga tokenizers om approximation räcker.

## SLO

- p95 router overhead < 100 ms.
- p99 router overhead < 250 ms.
- Error rate från router < 0.1 procent.
- Decision log enqueue success > 99 procent.

## Mätning

Varje request ska logga:

- `router_received_at`
- `decision_completed_at`
- `provider_request_started_at`
- `first_token_at`
- `response_completed_at`

## Optimeringsordning

1. Eliminera DB hits.
2. Kompilera policy.
3. Minska JSON copying.
4. Cachea registry.
5. Mät streaming overhead.
6. Skala workers för logs.
