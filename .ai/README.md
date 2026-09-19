# `.ai` Execution State

This directory is the canonical machine-readable execution state for the `main` checkout.

Use it for:

- selecting the next runnable task
- recording current in-progress, completed, and blocked work
- appending execution events to `ledger.jsonl`

Do not treat branch-local `.ai/` directories as source of truth.
