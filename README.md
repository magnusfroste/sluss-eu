# Sluss — the data-sovereign LLM gateway

[![CI](https://github.com/magnusfroste/sluss-eu/actions/workflows/ci.yml/badge.svg)](https://github.com/magnusfroste/sluss-eu/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](go.mod)
[![Container](https://img.shields.io/badge/ghcr.io-sluss--eu-24292e.svg)](https://github.com/magnusfroste/sluss-eu/pkgs/container/sluss-eu)

**Your teams already use AI. Decide where the data goes — and prove it.**

Sluss sits between your AI clients and the LLM providers. Every prompt is
classified with deterministic rules (**no LLM in the decision**; an optional
model on your own hardware may only make it stricter), routed under a
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

**It is an address, not a proxy.** Nothing sits in your network traffic and
nothing is installed on anyone's computer. You give your AI tools Sluss's
address instead of the AI company's (a base URL and a key — every serious AI
tool has the setting), and the tools you already run close the other doors:
**Entra ID** decides who may use AI and from which device, **Intune** pushes
the address to every computer and checks daily that it holds, and your **DNS
filter** blocks the AI websites and points people to the company chat.
[One address, closed doors](manual/one-address.md) explains it in five minutes.

Built for organisations under **NIS2, DORA and GDPR** — but useful to anyone
who wants to say *yes* to AI on their own rules. It sells **control and
evidence**; it makes no legal-compliance claims.

## Why rules, not another AI

**Sluss is the AI gateway whose decisions you can read.** It decides with
rules an auditor can read, not with another model: the same question gives
the same decision, every time, which is what evidence needs; the decision
takes under a millisecond; nothing leaves your machine to be classified; and
there is no second AI to govern, prompt-inject or explain.

That is how Sluss starts, and an organisation that wants no AI in the
decision can leave it that way and prove it: the log at start, `/version`,
the Version page and a `router.started` record in the audit log all say
*rules only*. If you want more, a small model on your own hardware can give a
second opinion that may only make a decision stricter
([How Sluss decides](manual/how-sluss-decides.md)).

The price is stated, not hidden: rules find identifiers, names, keys,
markings, terms and code, not meaning. A text that is sensitive only because of what it
says passes as harmless. [What Sluss catches](manual/what-sluss-catches.md)
measures it on a fixed set of examples that runs with every change, and the
[benchmark](manual/benchmark.md) scores 444 labelled questions, false alarms
included, beside an open-source PII detector, with a template to score your
current gateway the same way. You close
the gap with your own terms, with *default to local*, and, if you turn it on, with a
local model that may only make a decision stricter, never looser.

## Try it in 60 seconds

No provider key needed: until a real model is connected, a built-in test answer says so.

```bash
git clone https://github.com/magnusfroste/sluss-eu.git && cd sluss-eu
make build

# The gateway with the NIS2 pack live. The tags mark the test providers as
# "local" and "cloud" so the demo below has somewhere local to route to.
LOCAL_API_KEY=local_router_key ROUTER_POLICY_PATH=builtin:nis2-baseline \
  ROUTER_PROVIDER_TAGS="anthropic=local,eu-resident;openai=cloud" ./bin/router
```

Now watch the signature moment — the same client, two prompts, two destinations:

```bash
# Harmless → routed for cost
./bin/routerctl -url http://localhost:8080 -key local_router_key \
  -message "write a git commit message for this diff"

# Contains a Swedish personal ID (checksum-validated, not just a regex) → LOCAL only
./bin/routerctl -url http://localhost:8080 -key local_router_key \
  -message "summarise the case for customer 811218-2392"
```

`routerctl` is a **dry run**: it shows which rule fired, which model was chosen,
local or cloud, and why — without calling any provider. The same dry-run is in
the admin console and as the MCP tool `route_explain`.

**To open the console**, give Sluss a data directory and a first admin account
(both are needed for named sign-in), then sign in at `/router/login`:

```bash
LOCAL_API_KEY=local_router_key ROUTER_POLICY_PATH=builtin:nis2-baseline \
  ROUTER_DATA_DIR=./data ROUTER_ADMIN_USER=admin ROUTER_ADMIN_PASSWORD=change-me-now ./bin/router
```

Then open **http://localhost:8080** — a short home page with **Sign in** and
getting-started steps, `/connect` (per-client recipes), and the admin console,
where **Quick start** adds your first real models. Employees sign in with the
company account (OpenID Connect against Entra ID, Okta, Google or Keycloak);
administrators can also have local console accounts. Every chat request is
logged under the person and their department. Product information lives on
[www.sluss.eu](https://www.sluss.eu); a running instance tells search engines
not to index it.

Prefer a container? Releases are published as `ghcr.io/magnusfroste/sluss-eu:v1.10.2`
(pin a release in production — see [CHANGELOG](CHANGELOG.md)); every push to `main` publishes
`ghcr.io/magnusfroste/sluss-eu:latest` (public); `deploy/docker-compose.yml` is a
one-service compose with a data volume. Step by step — `docker run`, TLS,
upgrades, backup, air-gapped installs:
[docs/07-operations/self-host-docker.md](docs/07-operations/self-host-docker.md).

## What it does

| | |
|---|---|
| **Classify** | Task type, risk, and data sensitivity per prompt in < 1 ms: `secrets_possible` › `pii` (personnummer, e-mail, phone, address, card, IBAN — checksum-validated where a checksum exists) › `security_classified` › `health` › `financial` › `legal` › `source_code`. English + Swedish terms. Every class found counts, not only the strongest. Attached documents are classified as content; unreadable attachments form their own class. |
| **Second opinion** *(optional, off by default)* | A small model on your own hardware reads the question after the rules and may add a class they missed — a name with a story, an illness in plain words — never remove one. A cloud model is refused. The mode is in the banner, `/version` and the audit log. |
| **Gate agents** | Declared tools are classified `read` ‹ `write` ‹ `external` ‹ `destructive` (unknown → `write`, never `read`). Policy can keep a delete-capable agent on-prem or block it — before any model sees the prompt. |
| **Route by policy** | Firewall-style rules with three outcomes: **keep on own hardware**, **mask and send**, **block**. The strictest matching rule wins. Constraints are **fail-closed**: if no provider carries the required tag, the request is a 403 with an audit entry — never a silent cloud fallback. Client model pins cannot bypass policy. |
| **Mask and send** | Cloud quality without the identifiers: personal identity numbers, e-mail, phone, IBAN, card, address and keys become placeholders (`[EMAIL_1]`) before a cloud model sees the question, and the values are put back in the answer — streamed answers too. Only the types and counts are logged. |
| **Check answers** | Every answer is read with the same detectors as the question. A key or personal identifier the question did not hold is flagged in the request log and the audit log — a model that leaks training data, a tool that returns a customer's number, a prompt injection that repeats something it should not. |
| **Prove it** | Every decision explainable in one line, and testable in a dry-run that calls no model. Hash-chained audit export + offline verifier (`cmd/audit-verify`), anchored hourly in your log so a rewritten file is caught. Incident evidence for NIS2's 24h/72h windows on one URL. Audited per-tenant GDPR erasure — you can delete data, never the fact that you deleted it. |
| **Company chat** | A chat in the browser, signed in with the company account, with a history per person, attachments, optional web search through your own SearXNG, and an inspector that shows for every answer what was found, which rule matched, which model answered and what it cost. |
| **Built to be audited** | A login brake and browser hardening headers, `/metrics` behind a key, images signed with Sigstore and shipped with an SBOM, `govulncheck` on every change. |
| **Save** | Cheap tasks go to cheap or local models automatically; the dashboard shows spend and estimated CO₂e against an all-premium baseline (and names the baseline model). |

**Compliance packs** are embedded rulesets you adapt like a firewall's defaults:
`builtin:nis2-baseline`, `builtin:dora-baseline`, `builtin:gdpr-sovereign`,
`builtin:pii-local`. Or click rules together in the console — no YAML.

**Monitor mode** is the safe start: run a pack as the *shadow* policy for two
weeks, nothing blocks, and `GET /router/gap-report` tells you how many prompts
with personal data went to the cloud — and what the policy would have done.

## See it with real questions

[`examples/use-cases`](examples/use-cases/README.md): fifteen people and agents — marketing, a developer who pastes a key, customer support, HR, finance, legal, a defence supplier, agents with read-only and destructive tools — ask real questions under one policy with all three rule outcomes. `make use-cases` runs them against a fresh instance in seconds, with no key, and checks every outcome.

Evaluating Sluss for your organisation, or asking an agent to? [EVALUATE.md](EVALUATE.md) is the brief: the claims to test, what to run, what to try to break. We encourage it.

## Point your clients at it

Three values — base URL, API key, `model: auto`:

- **OpenAI wire** (`/v1/chat/completions`): Cursor, Cline, Continue, Aider, OpenWebUI, AnythingLLM, LibreChat, the OpenAI SDKs.
- **Anthropic wire** (`/v1/messages`, tool use + streaming): Claude Code, Codex, the Anthropic SDKs — `ANTHROPIC_BASE_URL=https://your-gateway`.
- **Agents**: an MCP surface (`/mcp`) exposes `route_explain`, `incident_report`, `savings_report`, `recent_requests` and more (read-only by default), so an agent can write your morning security brief from the gateway's own evidence.

`/connect` on a running instance has copy-paste recipes per client.

## Fits your IT — no new island

An IT organisation does not want another tool with its own accounts, its own
agent and its own way of being rolled out. Sluss plugs into what you already
run, the same way ChatGPT Enterprise does, and leaves the decisions where they
belong:

| Your system | What it does for Sluss |
|---|---|
| **Identity provider** (Entra ID, Okta, Google, Keycloak) | *Sign in with the company account* over OpenID Connect. Sluss keeps no employee accounts: your IdP decides who, MFA, device compliance (Conditional Access) and when someone leaves. Groups map to the admin console or chat-only, and to the department every request is logged under. |
| **MDM** (Intune, Jamf, Kandji, Fleet) | The **Rollout** page generates what the MDM runs: scripts that point every tool with a base URL at Sluss, a web clip that puts the company chat on every home screen (phones too), a browser policy, and a daily read-only check per device. |
| **DNS filter / secure web gateway** (Umbrella, Zscaler, Cloudflare Gateway) | A curated deny list of direct generative-AI domains, and a block page on your instance to point the sinkhole at: *"blocked here — open the company AI chat"*. Block + offer, no interception. |
| **Audit and incident process** | A hash-chained audit log with an offline verifier, 24h/72h incident evidence on one URL, and a dashboard that says how many managed devices can still reach a provider directly. |

Nothing of ours runs on the desktop, nothing inspects traffic.

**Manual.** [`manual/`](manual/) is the step-by-step guide for an administrator without specialist training — models and the first rule, stopping ChatGPT, developer tools, showing that it works, a glossary and troubleshooting. A running instance serves it at `/manual` with its own addresses filled in (and as raw Markdown at `/manual/<page>.md` for agents). The control is
*the only AI path is Sluss*; the evidence is per request and per device.

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
manual/           the administrator's manual — also served at /manual on every instance
docs/             how it is built (map: docs/README.md)
  00-product/     positioning, NIS2 controls, market + ecosystem research, product brief
  01-architecture/  system design       02-adr/   architecture decisions
  05-issues/      the backlog, as shipped (ISSUE-001 → …)   06-engineering/  references
  07-operations/  runbooks, SLOs, the open-source release runbook
DECISION_LOG.md   dated product/tech decisions
AGENTS.md         for agents: operating Sluss first, then changing the code
llms.txt          index for agents and LLMs
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
