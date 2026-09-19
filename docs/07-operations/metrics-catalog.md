# Metrics catalog

Faktiskt exporterade metrics (`internal/metrics/metrics.go`, Prometheus-namespace
`router`, skrapas på `GET /metrics`):

## Counters

- `router_requests_total{route_class, model, provider, status}`
- `router_fallbacks_total{provider, reason}`
- `router_event_queue_dropped_total`

## Histograms

- `router_routing_overhead_ms` — routingbeslut före provideranrop (SLO p95 < 100 ms)
- `router_first_token_ms{model, provider}`
- `router_provider_duration_ms{model, provider, success}`

## Gauges

- `router_event_queue_backlog`
- `router_provider_health_score{provider}`

## Labels

Använd labels försiktigt för cardinality.

Undvik:

- Raw tenant id i Prometheus om många tenants.
- Request id.
- Promptinnehåll.

Nya metrics läggs till i `internal/metrics/metrics.go` och dokumenteras här i
samma ändring — kataloger som listar metrics som inte exporteras ger döda
dashboards/larm.
