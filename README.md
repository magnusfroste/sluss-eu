# Sluss — the data-sovereign LLM gateway

[![CI](https://github.com/magnusfroste/sluss-eu/actions/workflows/ci.yml/badge.svg)](https://github.com/magnusfroste/sluss-eu/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8.svg)](go.mod)
[![Container](https://img.shields.io/badge/ghcr.io-sluss--eu-24292e.svg)](https://github.com/magnusfroste/sluss-eu/pkgs/container/sluss-eu)

**Your teams already use AI. Decide where the data goes — and prove it.**

Sluss sits between your AI clients and the LLM providers. Every prompt is
classified with deterministic rules (**no LLM in the decision**), routed under a
**fail-closed** policy — sensitive data to your own or an EU model, harmless
traffic to the cheapest capable cloud model — and recorded in a
**hash-chained audit log** an auditor can verify offline.

One static Go binary. SQLite. OpenAI **and** Anthropic wire formats. Runs in
*your* infrastructure — we never see a prompt.

```
Any AI client ──▶ SLUSS (your infra) ──▶ cloud models        (harmless)
 Cursor, Claude Code,   classify · route · prove   ──▶ your on-prem models (sensitive)
 OpenWebUI, agents…                                ──▶ BLOCKED, audited   (nothing compliant)
```

Built for organisations under **NIS2, DORA and GDPR** — but useful to anyone
who wants to say *yes* to AI on their own rules. It sells **control and
evidence**; it makes no legal-compliance claims.

## Try it in 60 seconds

```bash
git clone https://github.com/magnusfroste/sluss-eu.git && cd sluss-eu
make build

# Terminal 1 — a mock provider (answers like an OpenAI-compatible model)
MOCK_PROVIDER_ADDR=:18080 ./bin/mock-provider

# Terminal 2 — the gateway with the NIS2 pack live
LOCAL_API_KEY=local_router_key ROUTER_POLICY_PATH=builtin:nis2-baseline ./bin/router
```

Now watch the signature moment — the same client, two prompts, two destinations:

```bash
# Harmless → routed for cost
./bin/routerctl -url http://localhost:8080 -key local_router_key \
  -message "write a git commit message for this diff"

# Contains a Swedish personal ID (checksum-validated, not just a regex) → LOCAL only
./bin/routerctl -url http://localhost:8080 -key local_router_key \
  -message "summarise the case for customer 811218-9876"
```

`routerctl` is a **dry run**: it shows which rule fired, which model was chosen,
local or cloud, and why — without calling any provider. The same dry-run is in
the admin console and as the MCP tool `route_explain`.

Then open **http://localhost:8080** — a short home page with **Sign in** and
getting-started steps, `/connect` (per-client recipes), and the admin console
(log in with `ROUTER_DASHBOARD_PASSWORD` if set). Product information lives on
[www.sluss.eu](https://www.sluss.eu); a running instance tells search engines
not to index it.

Prefer a container? Every push to `main` publishes
`ghcr.io/magnusfroste/sluss-eu:latest`; `deploy/docker-compose.yml` is a
one-service compose with a data volume.

## What it does

| | |
|---|---|
| **Classify** | Task type, risk, and data sensitivity per prompt in < 1 ms: `secrets_possible` › `pii` (incl. personnummer) › `security_classified` › `health` › `financial` › `legal` › `source_code`. English + Swedish terms. Attached documents are classified as content; unreadable attachments form their own class. |
| **Gate agents** | Declared tools are classified `read` ‹ `write` ‹ `external` ‹ `destructive` (unknown → `write`, never `read`). Policy can keep a delete-capable agent on-prem or block it — before any model sees the prompt. |
| **Route by policy** | Firewall-style rules: `block → force → constraints → defaults`. Constraints accumulate and are **fail-closed**: if no provider carries the required compliance tag, the request is a 403 with an audit entry — never a silent cloud fallback. Client model pins cannot bypass policy. |
| **Prove it** | Every decision explainable in one line. Hash-chained audit export + offline verifier (`cmd/audit-verify`). Incident evidence for NIS2's 24h/72h windows on one URL. Audited per-tenant GDPR erasure — you can delete data, never the fact that you deleted it. |
| **Save** | Cheap tasks go to cheap or local models automatically; the dashboard shows spend and estimated CO₂e against an all-premium baseline (and names the baseline model). |

**Compliance packs** are embedded rulesets you adapt like a firewall's defaults:
`builtin:nis2-baseline`, `builtin:dora-baseline`, `builtin:gdpr-sovereign`,
`builtin:pii-local`. Or click rules together in the console — no YAML.

**Monitor mode** is the safe start: run a pack as the *shadow* policy for two
weeks, nothing blocks, and `GET /router/gap-report` tells you how many prompts
with personal data went to the cloud — and what the policy would have done.

## Point your clients at it

Three values — base URL, API key, `model: auto`:

- **OpenAI wire** (`/v1/chat/completions`): Cursor, Cline, Continue, Aider, OpenWebUI, AnythingLLM, LibreChat, the OpenAI SDKs.
- **Anthropic wire** (`/v1/messages`, tool use + streaming): Claude Code, Codex, the Anthropic SDKs — `ANTHROPIC_BASE_URL=https://your-gateway`.
- **Agents**: an MCP surface (`/mcp`) exposes `route_explain`, `incident_report`, `savings_report`, `recent_requests` and more (read-only by default), so an agent can write your morning security brief from the gateway's own evidence.

`/connect` on a running instance has copy-paste recipes per client.

## Why this exists (and why it is open source)

It started as private inference — vLLM on our own GPUs, RAG on internal data,
everything behind Zero Trust. Then more teams, then finance processes, then a
fleet of agents. At that point you need one place that decides where every
prompt is *allowed* to go and proves what happened. Sluss is that place, built
because we needed it ourselves.

It is open source because a security product should be readable. "No LLM in
the decision, fail-closed, tamper-evident audit" are claims you can check in
this repository rather than take on trust. The whole thing is here — code,
architecture, ADRs, backlog and market research — built in the open.

## How it compares (honestly)

| | LiteLLM | Portkey | SSE / DLP suites | **Sluss** |
|---|---|---|---|---|
| Self-hosted, single binary | ✓ (Python) | hybrid | SaaS | ✓ |
| Content-aware deterministic routing | metadata only | — | per-app, not per-prompt | ✓ |
| Fail-closed egress per data class | — | — | — | ✓ |
| Agent capability gating (tool risk) | — | — | — | ✓ |
| Tamper-evident audit + offline verifier | — | — | — | ✓ |
| EU regime packs (NIS2 / DORA / GDPR) | — | — | — | ✓ |

If you only need a multi-provider proxy with budgets, LiteLLM is excellent.
Sluss is for the moment a security lead asks *"where does our AI data go, and
can you prove it?"*

## Architecture

```
POST /v1/chat/completions  (or /v1/messages)
  → Auth (per-department API keys, hashed at rest)
  → Feature extractor (rule-based, no LLM) → JobDescriptor
  → Policy engine (precompiled, in-memory) → constraints or fail-closed block
  → Routing engine (score on quality · cost · latency · health · compliance tags)
  → Provider adapter (OpenAI-compatible, Anthropic, local vLLM/on-prem)
  → Audit chain + spend/CO₂ accounting (async, off the hot path)
```

Routing overhead target: **p95 < 100 ms** before any provider call. Storage is
SQLite under `ROUTER_DATA_DIR` — no Postgres, no Redis, nothing else to run.

Start with [`docs/01-architecture/01-system-overview.md`](docs/01-architecture/01-system-overview.md).
Decisions live in [`docs/02-adr/`](docs/02-adr/); the policy DSL is specified in
[`docs/06-engineering/01-routing-policy-reference.md`](docs/06-engineering/01-routing-policy-reference.md).

## Repository map

```
cmd/              router · routerctl (dry-run CLI) · audit-verify · mcp · mock-provider
internal/         classifier · policy · engine · provider · history (SQLite) · server · audit
docs/
  00-product/     positioning, NIS2 controls, market + ecosystem research, product brief
  01-architecture/  system design       02-adr/   architecture decisions
  05-issues/      the backlog, as shipped (ISSUE-001 → …)   06-engineering/  references
  07-operations/  runbooks, SLOs, the open-source release runbook
DECISION_LOG.md   dated product/tech decisions
```

## Develop

```bash
make dev           # go run ./cmd/router on :8080
make test          # full suite, -race
make test-policy   # policy golden cases
make test-eval     # classifier eval smoke
make demo-ciso     # reproducible end-to-end demo against the mock
make lint
```

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first — the fast-path rules (no LLM,
fail-closed, never reorder a classification vocabulary) are non-negotiable.
Security issues: [`SECURITY.md`](SECURITY.md).

## Licence

**AGPL-3.0-only** — run it, modify it, self-host it; if you offer a modified
version as a network service, share your changes under the same terms.
See [`LICENSE`](LICENSE).

---

*Sluss (Swedish: lock chamber) — it passes what should pass, holds what should
not, one chamber at a time, and logs every passage.* · [sluss.eu](https://sluss.eu)
