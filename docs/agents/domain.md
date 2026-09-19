# Domain Docs

How engineering skills should consume this repo's domain documentation when exploring the codebase.

## Layout

This repo uses a repo-specific single-context layout. There is no required root `CONTEXT.md` or `CONTEXT-MAP.md` at the moment.

## Before Exploring, Read These

Start with the smallest set relevant to the task:

- `README.md` for product framing and recommended reading order
- `CLAUDE.md` and `AGENTS.md` for repo-specific agent behavior
- `00-product/` for product goals, personas, scope, and roadmap
- `01-architecture/` for system design and request lifecycle
- `02-adr/` for architecture decisions
- `06-engineering/` for technical references and delivery standards
- `05-issues/` for existing self-contained implementation tasks

For most subsystem work, begin with:

- `01-architecture/01-system-overview.md`
- The most relevant subsystem document under `01-architecture/`
- Any matching ADR under `02-adr/`
- Any matching implementation issue under `05-issues/`

## Domain Vocabulary

Use the repo's established terms in generated issues, PRDs, tests, and refactor proposals:

- `JobDescriptor`
- `RouteDecision`
- `NormalizedModelRequest`
- `NormalizedModelResponse`
- fast path
- slow path
- provider adapter
- tenant policy
- fallback chain
- route attempt

Prefer these project terms over generic synonyms unless the source docs introduce a new term.

## ADR Conflicts

If a recommendation or implementation contradicts an existing ADR, surface it explicitly instead of silently overriding it:

> Contradicts `02-adr/ADR-0007-...` because...

## Missing Context

If a skill expects `CONTEXT.md` or `docs/adr/`, map that expectation to this repo's `README.md`, `01-architecture/`, and `02-adr/` instead. Do not create new context docs unless the task specifically calls for resolving domain language or architectural ambiguity.
