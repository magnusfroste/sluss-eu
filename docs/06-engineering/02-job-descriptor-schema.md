# JobDescriptor schema

## Syfte

`JobDescriptor` är routerns interna representation av promptens krav.

## Schema

```json
{
  "request_id": "req_...",
  "tenant_id": "tn_...",
  "project_id": "prj_...",
  "task_type": "hard_code_debugging",
  "risk_level": "high",
  "sensitivity": "source_code",
  "prompt_tokens_estimate": 4800,
  "max_output_tokens_estimate": 2000,
  "requires_reasoning": true,
  "requires_code": true,
  "requires_tool_use": false,
  "requires_json_schema": false,
  "requires_large_context": false,
  "requires_vision": false,
  "latency_preference": "balanced",
  "quality_preference": "high",
  "budget_preference": "normal",
  "files_touched": ["src/auth/session.ts"],
  "keywords": ["debug", "production", "auth"],
  "router_mode": "auto",
  "explicit_model": null,
  "metadata": {}
}
```

## Fält

| Fält | Beskrivning |
|---|---|
| `task_type` | Klassificerad uppgift |
| `risk_level` | `low`, `medium`, `high`, `critical` |
| `sensitivity` | `none`, `source_code`, `pii`, `secrets_possible`, etc. |
| `requires_reasoning` | Kräver djupare resonemang |
| `requires_tool_use` | Requesten innehåller tools/function calling |
| `latency_preference` | `fast`, `balanced`, `quality` |
| `quality_preference` | `cheap`, `balanced`, `high` |
| `router_mode` | `auto`, `cheap`, `balanced`, `premium`, `disabled` |

## Skapande

Källor:

1. Request metadata.
2. Headers.
3. Prompttext.
4. Klient-SDK signaler.
5. Policy defaults.

## Viktigt

Metadata från klient är inte automatiskt betrodd. Policy kan kräva att vissa signaler omtolkas eller höjer risk, särskilt vid känsliga filnamn.
