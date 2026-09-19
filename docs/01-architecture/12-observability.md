# Observability

## Mål

Det ska gå att svara på:

- Vilken modell valdes?
- Varför valdes den?
- Vad kostade requesten?
- Hur lång tid tog routern?
- Hur lång tid tog providern?
- Fungerade fallback?
- Vilka policies matchade?
- Vilka tasktyper driver kostnad?

## Metrics

### Router latency

- `router_overhead_ms`
- `feature_extraction_ms`
- `policy_eval_ms`
- `decision_ms`
- `event_enqueue_ms`

### Provider latency

- `provider_first_token_ms`
- `provider_total_latency_ms`
- `provider_timeout_count`
- `provider_error_count`

### Cost

- `estimated_cost_usd`
- `actual_cost_usd`
- `cost_by_model`
- `cost_by_tenant`
- `cost_by_task_type`

### Routing

- `route_decisions_total`
- `route_by_task_class`
- `route_by_model_tier`
- `fallback_count`
- `escalation_count`
- `manual_override_count`

### Quality/outcome

- `accepted_count`
- `rejected_count`
- `test_passed_count`
- `verifier_failed_count`

## Tracing spans

```text
router.request
  router.auth
  router.feature_extraction
  router.policy_eval
  router.decision
  router.provider.execute
    provider.http
  router.response_normalize
  router.event_log
```

## Logs

Logga strukturerat:

```json
{
  "event": "route_decision",
  "request_id": "req_123",
  "tenant_id": "tn_1",
  "route_class": "high_risk_code",
  "selected_model": "premium-reasoning",
  "policy_version": "pv_2026_05_19",
  "explanation": ["auth file matched", "premium required"],
  "router_overhead_ms": 48
}
```

## Dashboardar

MVP-dashboard:

- Total spend.
- Spend per modell.
- Route distribution.
- Latency p50/p95/p99.
- Provider errors.
- Fallback rate.

Beta-dashboard:

- Cost per successful task.
- Modelljämförelse per taskklass.
- Policy rule hit rate.
- Budget per project.
- Quality trend.

## Alerting

- Router p95 overhead över mål.
- Provider error rate över tröskel.
- Fallback rate plötsligt hög.
- Budget threshold nådd.
- Policy reload failure.
- Event queue backlog växer.
