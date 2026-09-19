# 2026-06-12 - Runtime Policy Wiring Operability

## Context

ISSUE-063 added policy-gated context pipeline activation. The first implementation added the DSL and request-path gate, but review found the shipped router binary only loaded the built-in default policy and had no operator path for tenant/project policy enablement.

## What I Learned

A server-side policy feature is not operable just because tests can inject a custom `policy.Cache`. The production bootstrap must expose a real runtime policy source and wire optional dependent components, otherwise the feature remains effectively disabled outside tests.

## Reuse Rules

- When adding policy-controlled runtime behavior, wire the policy source in `cmd/router`, not only in package tests.
- Keep the default policy safe, but provide an explicit operator path such as a policy file env var.
- Add bootstrap-level tests for policy-loader helpers and production fan-out wiring when package tests use injected caches.
- If a feature has a dashboard/eventlog consumer, wire the in-process tracker into the event fan-out in the binary.

## Failure Signals

- Unit tests pass by injecting a custom `PolicyCache`, but `cmd/router` always uses a built-in policy with no enabling rules.
- New optional server config fields exist but are never set by the shipped binary.
- Dashboard sections stay empty because the event handler that populates them is not in `eventlog.MultiHandler`.
- CLI artifacts or logs use generic labels even when operators provided named inputs.

## Next Checklist

- [ ] Check `cmd/router/main.go` for every new server config field.
- [ ] Add at least one package-main test for runtime loaders or wiring helpers.
- [ ] Keep default behavior disabled unless an operator explicitly enables the feature.
