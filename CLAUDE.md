# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Sluss (sluss.eu, module `github.com/magnusfroste/sluss`) is a self-hosted, data-sovereign LLM gateway written in **Go**, for EU organisations under NIS2/DORA/GDPR. It sits between AI clients and LLM providers, speaks both the OpenAI and Anthropic wire formats, classifies every prompt with deterministic rules (task type, risk, data sensitivity — never an LLM in the decision), routes it under a fail-closed policy, and records every decision in a hash-chained audit log. Positioning: **control + evidence** — never legal-compliance claims. The buyer is a CISO/DPO, not a developer optimizing cost.

The design target is routing overhead p95 < 100 ms before any provider call.

## Development commands

```bash
cp .env.example .env   # optional; runs with defaults
make dev               # go run ./cmd/router — listens on :8080
make run-mock          # local mock provider (no OPENROUTER_API_KEY needed)
```

Storage is SQLite (modernc, no CGO) under `ROUTER_DATA_DIR` — no external services required. The model registry is built from the SQLite roster only when `OPENROUTER_API_KEY` is set; otherwise a mock/default snapshot is used.

Run tests:
```bash
make test              # unit + integration
make test-unit         # unit only
make test-policy       # policy golden cases
make test-eval         # eval smoke (50+ prompt cases)
go test ./internal/classifier/... -run TestFeatureExtraction
```

Lint:
```bash
make lint
```

Dry-run a routing decision without calling a provider (also available as the MCP tool `route_explain` and `POST /router/policy/dryrun` in the policy console):
```bash
curl -X POST http://localhost:8080/router/decision \
  -H "Authorization: Bearer local_router_key" \
  -d '{"messages":[{"role":"user","content":"Summarise this contract"}]}'
# or: go run ./cmd/routerctl -key local_router_key -message "Summarise this contract"
```

## Architecture

### Request lifecycle

```
POST /v1/chat/completions   (or POST /v1/messages — Anthropic format)
  → Auth (API key → tenant + policy context, cached)
  → Feature Extractor (rule-based, no LLM) → JobDescriptor
  → Policy Engine (precompiled, in-memory) → constraints or fail-closed block
  → Routing Decision Engine (score candidates) → RouteDecision
  → Provider Executor (format translation + HTTP) → normalized response
  → Durable request log + audit chain (async, best-effort)
  → Client
```

Response headers carry routing metadata: `x-router-request-id`, `x-router-selected-model`, `x-router-route-class`, `x-router-route-reason`, `x-router-egress`.

### Key types

**`JobDescriptor`** — the internal contract produced by feature extraction. Fields include `task_type`, `risk_level`, `sensitivity`, `prompt_tokens_estimate`, `requires_reasoning`, `requires_tool_use`, `latency_preference`, `quality_preference`, `router_mode`. Defined in `docs/06-engineering/02-job-descriptor-schema.md`.

**`RouteDecision`** — output of the routing engine: selected model, fallback chain, timeout, verifier flag, decision reasons, policy version. Defined in `docs/01-architecture/04-routing-engine.md`.

**`NormalizedModelRequest` / `NormalizedModelResponse`** — internal wire format between router and provider adapters. Each provider has its own adapter that maps to/from this format.

### Core subsystems

**Feature extractor** — pure rule-based signal extraction (token count, code detection, keywords, file names, stack traces, SQL/auth/payment terms). Must never call an LLM. Target: p95 < 20 ms.

**Policy engine** — YAML policy compiled to in-memory rules. Evaluation order: block → force → constraints → hints → defaults. Rules match on `task_type`, `risk_level`, `sensitivity`, `prompt_tokens_gt/lt`, `contains_any`, `any_file_matches`, etc. Policy can be rolled back independently of code (hotreload).

**Routing algorithm** — filters candidates by capability + provider health, then scores:
```
score = quality_weight * predicted_quality
      + capability_weight * capability_match
      + health_weight * provider_health
      - cost_weight * estimated_cost
      - latency_weight * expected_latency
      - risk_penalty_for_underpowered_model
```
For risky tasks, fallback is always upward (more capable), never downward.

**Provider adapters** — one `ProviderAdapter` interface per provider (OpenAI, Anthropic, etc.). Responsible for request format translation, streaming chunk parsing, tool call mapping, token usage normalization, and error normalization to internal error types (`provider_timeout`, `provider_rate_limit`, `provider_5xx`, etc.).

**Sensitivity classification** — deterministic keyword/pattern rules (EN + SV) produce sensitivity classes: `secrets_possible` > `pii` (incl. Swedish personnummer) > `security_classified` > `health` > `financial` > `legal` > `source_code`. Array-form message content is classified as document text; unreadable attachments (images/binaries) set `requires_vision` and form their own policy class. Ranking order must never change so deployed policies keep matching.

**Agent capability gating** — declared tools (`tools` in the request, both OpenAI and Anthropic shapes) are classified deterministically into `read` < `write` < `external` < `destructive`; unknown tools default to `write` (fail-safe, never `read`). The highest class lands on `JobDescriptor.tool_risk` with `tools_declared` (names only — never arguments), and policy gates on `tool_risk` / `any_tool_matches`. Classification happens on the declaration, before the provider call, so fail-closed still holds.

**Compliance surface** — builtin policy packs (`builtin:pii-local`, `builtin:nis2-baseline`, `builtin:dora-baseline`, `builtin:gdpr-sovereign`), no-YAML console rules (stored in SQLite, hot-reloaded, audited), monitor mode / shadow policy + gap report, hash-chained audit export + offline verifier (`cmd/audit-verify`), incident evidence export (24/72h), audited GDPR tenant erasure, provider risk register (compliance tags gate egress, fail-closed).

### Infrastructure

- **SQLite** (modernc, no CGO, `ROUTER_DATA_DIR`) — the only store: tenants/keys, request history, model roster, console rules, audit chain, spend. Single static binary; runs anywhere.
- **In-memory caches** — compiled policy, registry snapshot, auth context. No Redis.
- **Async logging** — decision/attempt logging is non-blocking; failures here must not affect the request path.
- Postgres/Redis appear in `docker-compose.yml` only as a reserved Enterprise-scaling option — nothing in the runtime uses them.
- **MCP surface** at `/mcp` — route_explain, get_policy, get_roster, provider_probe, recent_errors, incident_report; write tools gated by `ROUTER_MCP_WRITE_ENABLED`.

### Task classes → model tiers

| Task class | Tier |
|---|---|
| `trivial_git`, `simple_shell` | cheap/local |
| `summarization`, `simple_code_edit` | balanced |
| `hard_code_debugging` | premium-reasoning |
| `security_review`, `database_migration` | premium + verifier |
| `long_context_analysis` | long-context model |
| `unknown_high_risk` | premium |

### Documentation structure

```
docs/
  00-product/       Positioning (08), NIS2 controls (09), market (10), ecosystem (11), product brief (13), agent market sweep (14)
  01-architecture/  System design (15 docs) — start here for any subsystem
  02-adr/           Architecture decisions (12 ADRs)
  03-backlog/       Epics (EPIC-01 through EPIC-10)
  04-sprints/       Sprint plans (sprints 0–8)
  05-issues/        Implementable issues (ISSUE-001 and up; see issue-index.md)
  06-engineering/   Technical references: routing policy, classifier, latency, testing, CI/CD
  07-operations/    Runbooks, SLO, incident response
  08-templates/     ADR, issue, epic, sprint, postmortem templates
```

Start at `docs/01-architecture/01-system-overview.md` for any new subsystem. Each ISSUE file in `docs/05-issues/` is a self-contained implementation task.

## Implementation constraints

- The fast-path routing decision must never call an LLM or external service — only in-memory data structures.
- Fallback chain must be constructed **before** the first provider call.
- Client metadata is not trusted — policy can override or escalate risk signals.
- Streaming fallback is only allowed before first token; after first token, restart requires explicit client opt-in.
- Policy and registry can be reloaded without a code deployment.

## Agent skills

### Issue tracker

Issues and PRDs are tracked as local markdown under `.scratch/<feature-slug>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

This repo uses the default five-state triage vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This repo uses a repo-specific single-context layout based on README, product docs, architecture docs, ADRs in `docs/02-adr/`, and engineering references. See `docs/agents/domain.md`.

## Status reporting (owner preference)

When reporting progress to the owner (who often replies just "kör"), always
end with a compact sprint/issue status table: each issue with ✅/⬅️ next/väntar,
one line on what just shipped and why it matters for the pitch, and name the
next issue so a bare "kör" is unambiguous. Lead with the outcome, keep it
readable on a phone.

## App language (owner decision 2026-07-10)

ALL new user-facing app text (admin UI, reports, error messages, demo chat) is
written in **English** — EU-wide product. Existing Swedish app text is swept in
ISSUE-097; Swedish repo docs may remain for now.
