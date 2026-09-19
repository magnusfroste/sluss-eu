# 2026-06-12 - System Prompt Adapter Boundaries

## Context

ISSUE-053 added the first model-specific prompt adapter skeleton on the provider side. The feature needed to stay disabled by default, preserve fast-path behavior for non-target requests, and avoid reordering or late mutation of chat messages before provider calls.

## What I Learned

Treat the system prompt as the existing `system`-role messages already present in `NormalizedModelRequest.Messages`; do not inject a new synthetic system message in the skeleton path. Mutate a cloned request, then commit it only when at least one rule matches. For multi-message system prompts, prepend only to the first `system` message and append only to the last so the logical prompt can be wrapped without changing message order.

## Reuse Rules

- Open this note when adding prompt adapters, provider-specific message rewriting, or pre-provider request mutators.
- Prefer clone-then-commit request mutation so disabled and non-target paths remain byte-for-byte unchanged.
- Prefer wrapping the first and last existing `system` messages over inserting or reordering messages.
- Avoid reading adapter activation from client-controlled metadata or headers.

## Failure Signals

- A disabled adapter changes provider request messages.
- A non-target model/profile mutates the request anyway.
- User or tool messages move position after adapter application.
- A mutation changes the live request even when no rule matched.

## Next Checklist

- [ ] Confirm the adapter is disabled by default in the server config path.
- [ ] Confirm matched mutations run on a cloned request and only commit on success.
- [ ] Add focused tests for exact-model and profile-based targeting before widening rules.
