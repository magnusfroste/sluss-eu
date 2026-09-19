# MCP surface (read-only) — ISSUE-074

Sluss exposes its routing introspection and observability data as a
[Model Context Protocol](https://modelcontextprotocol.io) (MCP) surface, so an
agent can discover and call the tools with schemas instead of scraping HTTP
endpoints. Two audiences:

1. **Analysis agent** — query spend/savings, the roster and provider health
   conversationally against a deployed instance (HTTP transport).
2. **Dev agent** (e.g. Claude Code) — introspect routing decisions during
   development: *"why did this route to premium?"* (stdio transport).

**Read-only.** No tool mutates state. `route_explain` is a dry-run — it never
calls a provider. The request log stores no raw prompts (masking), so nothing
sensitive leaks through these tools.

## Tools

| Tool | Arguments | Returns |
|---|---|---|
| `route_explain` | `prompt` (string) or `messages` (OpenAI-style array) | `task_type`, `task_confidence`, `risk`, `sensitivity`, `tier`, `selected_model`, `provider_model_id`, `reasons`, `fallbacks`, `est_cost_usd`, `policy_version`, and a `signals` block (keywords, files, requires_reasoning/code/tool_use, token estimate) — the same dry-run path as `POST /router/decision`. The signals turn *"it routed wrong"* into *"aha, it read the wrong signal"*. |
| `savings_report` | `window` (reserved; all-time for now) | saved USD & %, spend, tokens, estimated energy/CO₂e saved, route distribution — the same builder as the dashboard. |
| `recent_requests` | `n` (default 20, max 200), `task_filter` | recent request-log rows: time, task, risk, model, provider, tokens, estimated cost. |
| `get_roster` | — | tier → model with provider, `provider_model_id`, per-Mtok price, `enabled` and `reasoning_capable`, plus provider connections. |
| `provider_health` | — | per-provider health score in `[0,1]` and status (`healthy`/`degraded`/`down`). |
| `recent_errors` | `n` (default 20, max 200) | recent **upstream failures** since the last restart (in-memory): time, provider, model, `error_code` (`provider_5xx`/`provider_timeout`/`provider_auth_error`/…), attempt index, latency. No prompt text. Turns *"why is it 502ing?"* into a query. |
| `provider_probe` | `provider_id` (string), `mode` (`models` default, or `chat`) | live-check a **configured** provider **from the router's own network**: status, latency, body snippet and a plain-language `diagnosis` (Cloudflare error page vs origin 5xx vs auth vs connect failure). Only configured providers can be probed (base_url from the roster) — no arbitrary URLs. |
| `key_usage` | `key_id` (string) | has a prospect tested? realized activity for the key's tenant: `created_at`, `revoked`, `request_count`, tokens, `last_activity`, `used`. Mint one tenant per prospect for a clean signal. |
| `demo_link_usage` | — | whether the shareable demo link is enabled, its open count and last-open time. |

### Write tools (opt-in: `ROUTER_MCP_WRITE_ENABLED=true`)

These **mutate** state, so they are registered only when `ROUTER_MCP_WRITE_ENABLED` is set (still admin-gated and audited). They let an agent drive prospect outreach — tokenizer is the *create + measure* backend; the CRM/email loop lives elsewhere (e.g. an external CRM).

| Tool | Arguments | Returns |
|---|---|---|
| `mint_key` | `tenant` (unique per prospect, required), `project`, `role`, `scopes` | a new per-prospect key: `key_id` + the plaintext `api_key` **shown once**. |
| `revoke_key` | `key_id` (string) | revokes the key (e.g. at trial end); `revoked` bool. |
| `create_demo_link` | — | generates/rotates the shareable demo link and returns its full `url`. |
| `explain_request` | `request_id` (string) | look up a past decision by its `x-router-request-id`: routed model/tier/provider, tokens, cost. No raw prompt (masking). |
| `get_policy` | `tenant_id` (optional; global default) | the active routing policy: policy version, registry version, rule count and settings. |
| `server_info` | — | diagnostics: registry version, roster source (sqlite/registry), provider & enabled-model counts, and active runtime flags (conservative mode, pricing sync, decision-cache TTL, budget) — *"is prod running the config I think?"*. |

Every tool wraps existing internal logic (`engine.Decide`, the dashboard-data
builder, `history.Recent`, the roster store, the health tracker) — there is no
duplicated routing or classification code.

## Transport 1 — stdio (local dev agents)

Build the binary and point your agent at it:

```bash
go build -o bin/mcp ./cmd/mcp
```

It speaks newline-delimited JSON-RPC over stdin/stdout. With `ROUTER_DATA_DIR`
set it reads the same SQLite history/roster the router uses; with no data dir it
introspects the built-in default registry (enough for `route_explain` and
`get_roster`). Logs go to stderr so they never corrupt the protocol stream.

### Claude Code

```bash
claude mcp add tokenizer -- /absolute/path/to/bin/mcp
# or, with the same data volume as the router:
claude mcp add tokenizer --env ROUTER_DATA_DIR=/data -- /absolute/path/to/bin/mcp
```

Then, in a session: *"Use route_explain to show why 'review this auth change for
security' routes the way it does."*

### Raw stdio example

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"route_explain","arguments":{"prompt":"fix this failing Go data-race test"}}}' \
  | ./bin/mcp
```

## Transport 2 — HTTP (deployed instance)

Enable it and it mounts at `POST /mcp` on the running router:

```bash
ROUTER_MCP_ENABLED=true ./bin/router
```

**Default: off.** It only mounts when `ROUTER_MCP_ENABLED` is truthy
(`1`/`true`/`yes`/`on`) and a routing engine is configured.

It implements MCP Streamable HTTP: the client `POST`s one JSON-RPC message and
receives the JSON-RPC response as the body (all tools are request/response, so no
SSE stream is needed).

### Auth

`/mcp` is gated behind the **same credentials as the rest of the service** — a
valid Bearer API key, or (when set) the `ROUTER_DASHBOARD_PASSWORD` presented as
a Bearer token or HTTP Basic password:

```bash
# API key
curl -s -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer $LOCAL_API_KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# route_explain
curl -s -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer $LOCAL_API_KEY" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"route_explain","arguments":{"prompt":"summarise this changelog"}}}'
```

An MCP HTTP client (analysis agent) is configured with the endpoint URL and the
Bearer token; the same key that authorises `/v1/chat/completions` works here.

## Notes

- The surface never touches the fast path; it only reads already-computed data.
- Over stdio, `provider_health` starts empty (a fresh process has no live health
  window) — use the HTTP surface on the running router for live health.
- The tool set and its logic live once in `internal/server/mcp.go`
  (`NewMCPServer`); both transports register the identical tools. The protocol
  itself is a small dependency-free implementation in `internal/mcp` so the
  static `CGO_ENABLED=0` build stays lean.
