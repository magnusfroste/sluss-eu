# Model registry

## Syfte

Model registry beskriver vilka modeller som finns, vad de kostar, vad de är bra på och vilka tekniska funktioner de stöder.

## Registryfält

```json
{
  "id": "balanced-coder",
  "provider": "provider_a",
  "provider_model_id": "provider-model-name",
  "tier": "balanced",
  "capabilities": {
    "chat": true,
    "streaming": true,
    "tool_calls": true,
    "json_schema": true,
    "vision": false,
    "long_context": false
  },
  "strengths": ["code", "summarization", "tool_use"],
  "weaknesses": ["hard_reasoning", "security_review"],
  "context_window_tokens": 128000,
  "cost": {
    "input_per_million": 1.0,
    "output_per_million": 3.0
  },
  "latency_profile": {
    "p50_first_token_ms": 700,
    "p95_first_token_ms": 1800
  },
  "quality_scores": {
    "simple_code_edit": 0.78,
    "hard_code_debugging": 0.55,
    "security_review": 0.48
  },
  "enabled": true
}
```

## Modellnivåer

| Nivå | Syfte |
|---|---|
| `local` | Nära nollkostnad, triviala uppgifter, låg risk |
| `cheap` | Billiga enkla prompts |
| `balanced` | Standardnivå för vanlig kod och analys |
| `premium` | Svåra, riskabla eller högvärdesuppgifter |
| `specialized` | Long context, vision, embeddings, code, JSON, etc. |

## Health fields

Separera statisk registrydata från dynamisk health:

```json
{
  "model_id": "balanced-coder",
  "provider": "provider_a",
  "health_score": 0.92,
  "recent_error_rate": 0.01,
  "recent_timeout_rate": 0.02,
  "p95_latency_ms": 2400,
  "rate_limited": false,
  "updated_at": "2026-05-19T10:15:00Z"
}
```

## Versionering

Registry ska versioneras. Varje routingbeslut ska logga:

- `model_registry_version`.
- `policy_version`.
- `router_version`.

Det gör att beslut kan reproduceras i efterhand.

## Uppdatering

MVP:

- Roster (providers + modeller) lagras i SQLite (ISSUE-073); registry byggs från rostern när `OPENROUTER_API_KEY` är satt, annars mock/default-snapshot.
- Reload via admin endpoint.

Beta:

- Provider health job.
- Prisuppdateringar manuellt eller via scheduler.
- Per-tenant modellallowlist.

## Viktig designregel

Använd inte marknadsföringsnamn direkt i policy. Policy ska referera till interna modellprofiler, t.ex. `premium-reasoning`, så att faktisk provider/modell kan bytas utan policyomskrivning.
