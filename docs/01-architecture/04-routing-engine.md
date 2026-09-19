# Routing engine

## Syfte

Routing engine väljer bästa modell/provider för varje prompt givet:

- Uppgiftens karaktär.
- Risk.
- Modellkapabilitet.
- Latencykrav.
- Budget.
- Providerhälsa.
- Tenantpolicy.
- Historiska outcomes.

## Input

Input är ett `JobDescriptor` plus tenant context.

```json
{
  "task_type": "code_debugging",
  "risk_level": "high",
  "prompt_tokens_estimate": 4800,
  "requires_reasoning": true,
  "requires_tool_use": true,
  "requires_large_context": false,
  "sensitivity": "source_code",
  "latency_preference": "balanced",
  "quality_preference": "high",
  "metadata": {
    "project": "billing-api",
    "files_touched": ["src/payments/checkout.ts"]
  }
}
```

## Output

```json
{
  "route_id": "route_01",
  "selected_model": "premium-reasoning",
  "selected_provider": "provider_a",
  "fallbacks": ["balanced-coder", "premium-reasoning-provider-b"],
  "timeout_ms": 30000,
  "requires_verifier": true,
  "decision_reason": [
    "Task classified as high_risk_code",
    "Payment-related file detected",
    "Quality preference high",
    "Policy requires premium tier"
  ],
  "policy_version": "pv_2026_05_19"
}
```

## Routingalgoritm v1

1. Läs explicit override.
2. Applicera policy constraints.
3. Filtrera kandidater på capabilities.
4. Filtrera bort providers med dålig health.
5. Beräkna score.
6. Välj primär modell.
7. Bygg fallbackkedja.
8. Logga beslut.

## Scoring

```text
score =
  quality_weight * predicted_quality
+ capability_weight * capability_match
+ health_weight * provider_health
- cost_weight * estimated_cost
- latency_weight * expected_latency
- risk_penalty_for_underpowered_model
- policy_penalty
```

## Fast path vs slow path

### Fast path

Används för majoriteten av requests.

- Regler.
- Tokenestimat.
- Metadata.
- In-memory policy.
- In-memory registry.

### Slow path

Används endast när beslutet är osäkert eller tasken är dyr/riskabel.

- Liten lokal classifier.
- Verifiermodell.
- Extra policy lookup.
- Human approval i enterprise-läge.

## Taskklasser

Initiala taskklasser:

| Taskklass | Typisk modellnivå |
|---|---|
| `trivial_git` | cheap/local |
| `simple_shell` | cheap/local |
| `summarization` | cheap/balanced |
| `simple_code_edit` | balanced-coder |
| `hard_code_debugging` | premium-reasoning |
| `security_review` | premium + verifier |
| `database_migration` | premium + verifier |
| `long_context_analysis` | long-context model |
| `creative_copy` | cheap/balanced |
| `unknown_high_risk` | premium |

## Konservatism

För riskabla uppgifter ska fallback vara uppåt, inte nedåt.

Exempel:

```text
trivial -> cheap
osäker trivial -> balanced
riskabel kod -> premium
okänd + känslig data -> blockera eller premium beroende på policy
```
