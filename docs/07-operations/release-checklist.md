# Release checklist

> För **beta-lansering** gäller den mer omfattande `beta-release-checklist.md`
> (security, latency, fallback + sign-off-process). Denna lista gäller löpande
> releaser efter beta.

## Före release

- Unit tests passerar.
- Integration tests passerar.
- Policy tests passerar.
- Eval smoke test passerar.
- SQLite-schema-ändringar (idempotenta ALTER vid boot) verifierade.
- Registry validerat.
- Provider credentials verifierade.
- Dashboard visar stagingdata.
- Rollbackplan finns.

## Efter release

- Kontrollera router p95 overhead.
- Kontrollera provider error rate.
- Kontrollera fallback rate.
- Kontrollera event queue backlog.
- Kontrollera spend aggregation.
- Kontrollera att decision logs skrivs.

## Rollback

- Kod rollback via tidigare image.
- Policy rollback via tidigare policyversion.
- Registry rollback via tidigare registryversion.
