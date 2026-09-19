# Contributing to Sluss

Thanks for looking under the hood. Sluss is built in the open because a
security product should be readable: "no LLM in the decision, fail-closed,
hash-chained audit" are claims you can verify in this repo.

## Ground rules

- **Deterministic fast path.** Nothing on the request path may call an LLM or
  an external service. Classification is keyword/pattern rules; routing is
  scoring over in-memory data. PRs that break this are declined regardless of
  how clever they are.
- **Fail-closed stays fail-closed.** A change that turns a block into a silent
  fallback is a security regression.
- **Never reorder classification vocabularies.** Sensitivity classes and tool
  risk classes are ranked, and deployed policies match on their names. Add new
  classes at the right rank; never rename or reorder existing ones.
- **Evidence, not content.** Logs and reports record counts, classes and
  decisions — never prompt content or tool-call arguments.
- **No secrets, ever.** CI runs `scripts/check-secrets.sh`; keys are referenced
  by env-var name only.

## Getting started

```bash
make dev        # run the gateway on :8080 (SQLite under ROUTER_DATA_DIR)
make run-mock   # optional local mock provider
make test       # full suite with -race
make test-policy
make lint
```

Start with `docs/01-architecture/01-system-overview.md`, then the subsystem
doc for the area you are touching. Each `docs/05-issues/ISSUE-*.md` is a
self-contained implementation record — the backlog is public on purpose.

## Making a change

1. Open an issue or pick one from `docs/05-issues/` / GitHub Issues.
2. Branch from `main`; keep PRs focused.
3. Add tests in the package that owns the contract. For policy or classifier
   changes, add a golden case.
4. If a contract changes, update the matching doc (`docs/06-engineering/`
   for the policy DSL and descriptor schema).
5. `gofmt`, `go vet`, `make test` green. CI must pass.

## Language

Code, comments, commit messages and user-facing app text are in **English**.
Some historical planning docs are in Swedish; that is fine, and translating
them is welcome.

## Licence

By contributing you agree that your contribution is licensed under the
project licence (AGPL-3.0-only). See `LICENSE`.
