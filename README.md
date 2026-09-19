# Sluss — data-sovereign LLM gateway

**Keep sensitive prompts in the house. Route the rest for less.**

Sluss is a deterministic, auditable LLM gateway (OpenAI- **and** Anthropic-compatible)
for organisations under **NIS2, DORA and GDPR**. It classifies every prompt with
rules — never an LLM — and routes it by policy: prompts containing personal data
or possible secrets go to a local/EU model (or are **blocked fail-closed**),
everything else goes to the cheapest capable model for cost and CO₂. Every
decision is explainable, and the control plane writes a **tamper-evident,
hash-chained audit log** an auditor can verify offline.

> Positioning discipline: Sluss sells **control and evidence** for the
> accountable owner (CISO/DPO/leadership). It is a tool — it makes **no legal
> compliance claims**.

## Why

Under NIS2 (directive 2022/2555, transposed across the EU — in Sweden as
cybersäkerhetslagen, in force 2026) management is personally accountable for
cyber risk management. AI usage is a data-egress risk: every prompt a developer
sends to a cloud model is data leaving the house. Sluss is the control point
where that egress becomes **governed, observable and provable** — with cost and
CO₂ savings as the built-in ROI.

## How it works

```
Client (model: "auto")
  → Auth (per-department API keys, hashed at rest)
  → Classifier (rule-based, <20ms p95, no LLM): task, risk, sensitivity, PII types
  → Policy engine ("firewall for data egress"): block → force → constraints → defaults
      constraints accumulate and are FAIL-CLOSED — never a silent cloud fallback
  → Routing engine: score candidates on quality/cost/latency/health/compliance tags
  → Provider adapter (OpenAI-compatible, incl. local vLLM/on-prem)
  → Tamper-evident audit + spend/CO₂ accounting (async, off the hot path)
```

Key properties:

- **Deterministic** — no LLM in the routing decision, so every choice is
  reproducible and auditable. Dry-run any prompt (`POST /router/decision`, the
  Policy console, or MCP `route_explain`) to see which rule fires — before a
  single provider call.
- **Fail-closed** — if policy requires a `local`/`eu-resident` provider and none
  is available, the request is blocked with an audited 403. Client model pins
  cannot bypass constraints.
- **Compliance packs** — embedded rulesets selected per market/regime, like a
  firewall's default rules: `builtin:nis2-baseline`, `builtin:dora-baseline`,
  `builtin:gdpr-sovereign`, `builtin:pii-local`. The control report and public
  label follow the active pack (regime profiles).
- **Monitor mode (the safe start)** — run a pack as the *shadow* policy:
  nothing blocks, nothing changes, but `GET /router/gap-report` counts what the
  pack *would* have done with sensitive prompts. Convert to enforcement by
  moving one env var.
- **Reasoning control** — mark a model "reasoning-capable" and the router flips
  vLLM's `enable_thinking` per request: simple tasks answer instantly, hard
  tasks think. One local model, deterministically controlled.

## Point your clients at it

Sluss speaks both wire formats, so nearly any client connects with three
values — base URL, API key, `model: auto`:

- **OpenAI-wire** (`/v1/chat/completions`): Cursor, Cline/Continue, Aider,
  OpenWebUI, AnythingLLM, LibreChat, the OpenAI SDKs…
- **Anthropic-wire** (`/v1/messages`, incl. tool use + streaming): Claude Code,
  Codex, the Anthropic SDKs — `ANTHROPIC_BASE_URL=https://your-router`.

See **`/connect`** on a running instance for per-client recipes.

## Quickstart

Requires Go 1.22+. No database needed — the fast path is in-memory.

```bash
make build

# Terminal 1: mock provider (answers like an OpenAI-compatible model)
MOCK_PROVIDER_ADDR=:18080 ./bin/mock-provider

# Terminal 2: the router, with the NIS2 pack live
LOCAL_API_KEY=local_router_key ROUTER_POLICY_PATH=builtin:nis2-baseline ./bin/router
```

Try it:

```bash
# Routed chat completion (model: auto → the router decides)
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer local_router_key" \
  -d '{"model":"auto","messages":[{"role":"user","content":"write a git commit message"}]}'

# Dry-run: which rule fires, which model, local or cloud — no provider call
./bin/routerctl -url http://localhost:8080 -key local_router_key \
  -message "summarize the case for customer 811218-9876"
```

Real models: set `OPENROUTER_API_KEY` (OpenAI-compatible aggregate) and/or add
your own providers (e.g. an on-prem vLLM box tagged `local`) on the Providers
admin page. The reproducible end-to-end demo: `make demo-ciso`.

Tests: `make test` (race), `make test-policy` (golden cases), `make test-eval`,
`make smoke` (full lifecycle against the mock), `make lint`.

## The admin console

Behind named-user login (PBKDF2, per-person audit attribution, break-glass env
password): **Dashboard** (egress & compliance, savings vs all-premium, CO₂
receipt) · **Policy** (active pack, compliance packs, dry-run box) · **Models /
Providers** (roster with compliance tags, `local`/`eu-resident`/…) · **Keys**
(per-department, hashed at rest) · **Users** · **Demo prompts + shareable demo
link** (least-privilege guest session) · **Live chat** (the visible cloud→local
switch). Reports: `GET /router/compliance/report` (control report, regime-
flavoured) and `GET /router/gap-report` (monitor mode).

Agents can introspect a running instance via **MCP** (`POST /mcp`, off by
default): `route_explain`, `provider_probe`, `recent_errors`, spend/roster/
health, and — behind an explicit write gate — key provisioning for prospect
trials. See [`docs/mcp.md`](docs/mcp.md).

## Deploy (Docker / EasyPanel)

One static binary, one container, configured entirely via env
(see `.env.example`, `deploy/example.env`).

- [ ] Strong `LOCAL_API_KEY` (`openssl rand -hex 24`) and `ROUTER_DASHBOARD_PASSWORD`.
- [ ] `ROUTER_DATA_DIR=/data` on a mounted volume — durable history, audit chain, roster, keys.
- [ ] Pick a policy pack: `ROUTER_POLICY_PATH=builtin:nis2-baseline` (or start in
      monitor mode: `ROUTER_SHADOW_POLICY_PATH=builtin:nis2-baseline`).
- [ ] `ROUTER_PUBLIC_URL=https://your-domain` (landing/connect links).
- [ ] Expose port 8080, healthcheck `/healthz`; verify with a routed call and
      check the `X-Router-*` response headers.

Details: [`docs/01-architecture/14-deployment-topology.md`](docs/01-architecture/14-deployment-topology.md).

## Documentation map

```
docs/
  00-product/       CISO positioning, NIS2 routing-controls analysis, EU/global
                    market analysis, ecosystem map, agent market sweep, product brief
  01-architecture/  System design (start: 01-system-overview.md)
  02-adr/           Architecture decisions
  03-backlog/       Epics · 04-sprints/ Sprint plans · 05-issues/ Implementable issues
  06-engineering/   Routing policy, classifier, latency, testing references
  07-operations/    Runbooks, SLO, release checklists
DECISION_LOG.md     Short product/tech decisions with dates
```

Recommended reading order: `docs/00-product/13-product-brief.md`
→ `docs/00-product/08-positioning-ciso.md`
→ `docs/00-product/09-nis2-routing-controls.md` → `docs/01-architecture/01-system-overview.md`.

## Stack

Go 1.22+, stdlib `net/http`, `log/slog`; pure-Go SQLite (`modernc.org/sqlite`)
for durable history on a volume — no CGO, one static binary. Prometheus metrics,
built-in dashboard. Provider adapters in `internal/provider` (OpenAI-compatible,
mock for dev). See `DECISION_LOG.md` for the storage/architecture decisions.

## Licence

Sluss is open source under the **GNU Affero General Public License v3.0**
(`LICENSE`). Run it, modify it, self-host it; if you offer a modified version
as a network service, share your changes under the same terms. See
`CONTRIBUTING.md` to get involved and `SECURITY.md` to report a vulnerability.
