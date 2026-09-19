# AGENTS.md

This file defines how agents should work in this repository.

## Project Context

Sluss (sluss.eu) is a self-hosted, data-sovereign LLM gateway written in Go for EU organisations under NIS2/DORA/GDPR. It speaks both the OpenAI and Anthropic wire formats, classifies every prompt with deterministic rules (never an LLM in the decision), routes it under a fail-closed policy, and records every decision in a hash-chained audit log. Positioning: control + evidence — never legal-compliance claims. The buyer is a CISO/DPO, not a developer optimizing cost.

The design target is routing overhead p95 under 100 ms before any provider call. The fast path must stay rule-based, in-memory, and free of LLM or external-service calls. Runtime storage is SQLite (`ROUTER_DATA_DIR`) — no Postgres or Redis.

## Bootstrap Flow

For substantive tasks, use this mental model:

```text
BOOTSTRAP -> RETRIEVE -> EXECUTE -> LEARN -> UPDATE STATE
```

This is a guideline, not a rigid ceremony. The rules below define when each part applies.

## Repo Orientation

Start with these files when a task touches the relevant area:

- Product and scope: `docs/00-product/13-product-brief.md` (what the product is), `docs/00-product/08-positioning-ciso.md` (strategy), `docs/00-product/09-nis2-routing-controls.md` (compliance controls)
- System shape: `docs/01-architecture/01-system-overview.md`
- Request path: `docs/01-architecture/03-request-lifecycle.md`
- Routing: `docs/01-architecture/04-routing-engine.md`, `docs/01-architecture/05-low-latency-architecture.md`
- Provider boundary: `docs/01-architecture/08-provider-abstraction.md`
- Policy/security: `docs/01-architecture/07-policy-engine.md`, `docs/01-architecture/11-security-privacy.md`
- Implementation tasks: `docs/05-issues/`
- Engineering references: `docs/06-engineering/`

Each issue file in `docs/05-issues/` is intended to be self-contained implementation input. Treat architecture and ADR docs as the source of product/engineering intent, then synchronize execution state in `.ai/`.

## Development Commands

Common commands:

```bash
make dev
make test
make test-unit
make lint
make fmt
```

There are no local service dependencies — storage is SQLite under `ROUTER_DATA_DIR`. Optional: `cp .env.example .env` for provider keys; `make run-mock` starts a local mock provider so nothing external is needed.

Use focused `go test` packages while iterating. Before claiming a substantive code change is complete, run the narrow relevant tests and, when feasible, the broader `make test` or `make lint` gate.

## Implementation Rules

- Keep routing decisions deterministic and fast-path safe.
- Do not call an LLM or external service during fast-path classification or routing.
- Construct fallback chains before the first provider call.
- Treat client metadata as untrusted. Policy can override or escalate risk signals.
- Streaming fallback is only allowed before first token unless the client explicitly opts in to restart behavior.
- Keep provider adapters behind internal normalized request/response contracts.
- Prefer existing Go standard-library patterns and small package boundaries over new framework dependencies.
- For shared behavior, add focused tests in the package that owns the contract.

## Agent skills

### Issue tracker

Issues and PRDs are tracked as local markdown under `.scratch/<feature-slug>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

This repo uses the default five-state triage vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This repo uses a repo-specific single-context layout based on README, product docs, architecture docs, ADRs in `docs/02-adr/`, and engineering references. See `docs/agents/domain.md`.

## Learning notes

Consult `docs/notes/index.md` for prior lessons relevant to the affected area. After substantive work, check for a reusable bug, architecture, state-sync, performance or UX lesson. If one exists, update the note and index together; otherwise leave the notes unchanged.

When writing a note or bootstrapping the notes structure, follow [the learning-note protocol](docs/agents/learning-notes.md) for the format and maintenance rules.

## Learning Retrieval

### Goal

Use prior learnings before substantive work instead of only writing new ones after the fact.

### Retrieval Rules

Before substantive work:

1. Read repo-local `AGENTS.md`.
2. Read repo-local `memory.md` if present for quick orientation only.
3. If `docs/notes/index.md` exists, scan it and open only task-relevant notes before planning or implementation.
4. If `docs/notes/index.md` does not exist yet, create the notes structure on the first substantive task.

Do not bulk-read every note by default.

### Authority Rules

- `docs/notes/` is the canonical learning source.
- `memory.md` is a quick operational summary only.
- If `memory.md` and `docs/notes/` disagree, prefer `docs/notes/` and update `memory.md` later if needed.

## Execution State

### Goal

Maintain machine-readable execution state so `continue` can work without the user selecting the next task manually.

### Canonical Location

1. Canonical execution state lives only in the `.ai/` directory of the checkout whose branch is `main`.
2. Branch-local or worktree-local `.ai/` is not source of truth and must not be used.
3. On a non-`main` branch or worktree, resolve the absolute path of the repo's `main` checkout before reading or writing execution state.
4. If the `main` checkout path cannot be resolved, stop and ask the user. Do not create, read, or update `.ai/` in the current non-`main` checkout.

### Bootstrap

On the first substantive task in this repo, ensure these files exist in the `main` checkout's `.ai/` directory:

- `.ai/state.json`
- `.ai/tasks.json`
- `.ai/ledger.jsonl`
- `.ai/README.md`

### Usage Rules

- Treat the `main` checkout's `.ai/` as the canonical execution truth for next-task selection, in-progress state, completion state, and blocked state.
- Ignore any branch-local `.ai/` as stale or invalid.
- Never create or update `.ai/` outside `main`.
- Do not infer live execution state from sprint or issue markdown when canonical `.ai/` exists.
- Markdown planning docs may remain human-readable input, but execution state must be synchronized into the main checkout's `.ai/tasks.json`.

### Read and Write Points

- Before asking what to do next, or when continuing multi-step execution, read the main checkout's `.ai/state.json` and `.ai/tasks.json`.
- After substantive execution work from any branch or worktree, update the main checkout's `.ai/state.json` and `.ai/tasks.json`, and append an event to `.ai/ledger.jsonl`.

### Fallback Rule

- Treat sprint and issue markdown as planning input unless canonical `.ai/` already exists.
- When branch-local files or markdown disagree with the canonical main checkout `.ai/`, prefer the canonical main checkout `.ai/`.
- If `.ai/state.json` has no `next`, determine the next runnable task from `.ai/tasks.json` and set it during the task flow.

## Browser Tool Preference

### Goal

Use the browser surface that best matches Codex app workflows, with the in-app browser as the default for interactive browser work.

### Default Rule

- In Codex app sessions, prefer the in-app browser for browser-based work by default.
- Treat the in-app browser as the first choice for `localhost`, local QA, visual inspection, manual clicking, and user-visible app flows.

### Use Playwright Only When Needed

- Use Playwright or other browser automation only when the task specifically needs scripted automation, strict reproducibility, repeated step execution, DOM/programmatic extraction at scale, or capabilities the in-app browser does not provide cleanly.
- If both approaches would work, choose the in-app browser first.

### Explicit Plugin Rule

- If the user explicitly asks to use the in-app browser, `@browser-use`, Browser Use, or the current in-app browser tab, do not substitute Playwright unless the user approves a fallback.
