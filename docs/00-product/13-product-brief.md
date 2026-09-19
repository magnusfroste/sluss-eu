# 13 — Product brief (leave-behind, v1 2026-07-18)

> The 2–3 page product description used as a leave-behind after CISO/investor
> meetings. Brand: Sluss (sluss.eu). Same honesty rules as everything else:
> control + evidence, never legal-compliance claims. A dark-styled PDF is
> generated from this file; regenerate after edits.

## Page 1 — What Sluss is

**Sluss — the data-sovereign LLM gateway.** *Say yes to AI — on your rules.*

Sluss is the control point between your organisation and the AI providers. It
is a self-hosted gateway (one static binary or container, in your own
infrastructure) that speaks both the OpenAI and Anthropic wire formats, so any
AI client — Cursor, Claude Code, chat UIs like OpenWebUI or AnythingLLM,
internal apps, SDKs, agents — connects by changing a base URL and a key.

Every prompt that passes through is:

1. **Classified** — in under a millisecond, with deterministic rules (no LLM,
   no ML in the decision): task type, risk level, and data sensitivity —
   personal data (incl. Swedish personnummer), possible secrets, financial,
   health, legal and security-classified content, in English and Swedish.
   Attached documents are classified as content; unreadable attachments
   (images, binaries) form their own policy class.
2. **Routed by policy** — firewall-style rules (condition → action) decide
   where the prompt is *allowed* to go: harmless traffic to the best-value
   cloud model, sensitive classes to your own on-prem/EU model, and when no
   compliant model exists the request is **blocked, fail-closed** — never a
   silent fallback. Users pinning a specific model cannot bypass policy.
3. **Proven** — every decision carries human-readable reasons and lands in a
   hash-chained, offline-verifiable audit log. Control reports, shadow-AI gap
   reports, and incident evidence for NIS2's 24/72-hour windows are one URL
   away. Per-tenant GDPR erasure is itself audited: you can delete data, but
   never the fact that you deleted it.

**Who it is for:** organisations under NIS2, DORA and GDPR — especially
Nordic/DACH regulated mid-market (banks, insurers, energy, healthcare,
municipal utilities) where the law applies but SSE/MDM tooling does not reach.

## Page 2 — How a rollout works (the maturity ladder)

**1. Measure (monitor mode)** — deploy the gateway, point sanctioned AI tools
at it, and run a compliance pack as a *shadow policy*: nothing blocks, nothing
changes for users. After two weeks the **gap report** answers the question no
one else can: *how many prompts containing personal data went to a cloud
model — and what would the policy have done instead?*

**2. Enforce** — flip the pack from shadow to active (one setting, no
redeploy), or click rules together in the **policy console** — no YAML.
Embedded packs for NIS2, DORA and GDPR-sovereignty are starting rulesets you
adapt like a firewall's defaults. Dry-run any prompt to see which rule fires
before you commit. Every activation and rollback is audited.

**3. Prove** — export the tamper-evident audit chain (verifiable offline),
hand the control report to your auditor, pull 24/72h incident evidence when
the clock is ticking, and answer the board's question — *"where does our AI
data go?"* — with data instead of hope.

**The economics:** cheap tasks route to cheap models automatically; the
dashboard shows spend and estimated CO₂e against an all-premium baseline. The
savings typically self-fund the licence.

## Page 3 — Why Sluss over the alternatives

**Zero Trust, applied to every prompt** — verify explicitly (deterministic
per-prompt classification), least privilege (cheapest/most-local capable
model, residency-tag-gated egress), assume breach (fail-closed + tamper-evident
evidence). And taken one step further than SaaS can follow:

> **"True zero trust includes your vendors. The gateway runs in your
> infrastructure — we never see a prompt."**

**What no one else combines:** self-hosted deployment · content-aware
deterministic routing · fail-closed egress per prompt class · tamper-evident
audit · EU regime packs. Generic gateways optimise traffic; SSE tools control
apps, not prompt-to-model routes; SaaS "AI security" requires trusting another
cloud. Sluss decides where your data is *allowed* to go — and proves it.

**Supply-chain visibility built in:** a provider risk register records the
compliance facts you assert (DPA signed, ISO 27001, SOC 2, residency), warns
when the active policy requires a tag no provider carries, and audits every
change.

**Deployment:** single static Go binary or container · SQLite storage, no
external dependencies · runs on-prem, in your VPC, or any EU host · policy
and model roster change without redeploying code · OpenAI + Anthropic
compatible APIs · MCP tools for agent-driven operations and live debugging.

*Sluss — a lock chamber for your AI traffic: it passes what should pass,
holds what should not, one chamber at a time — and logs every passage.*
*Contact: Magnus Froste · sluss.eu*
