# Backlog index

## Epics

1. `EPIC-01-proxy-foundation.md` — OpenAI-kompatibel proxy.
2. `EPIC-02-model-registry.md` — Modell- och providerregistry.
3. `EPIC-03-classifier-och-jobdescriptor.md` — Feature extraction och JobDescriptor.
4. `EPIC-04-policy-engine.md` — Policy engine.
5. `EPIC-05-routing-decision-engine.md` — Scoring och routebeslut.
6. `EPIC-06-provider-execution-och-fallback.md` — Provider execution, streaming och fallback.
7. `EPIC-07-observability-och-dashboard.md` — Logging, metrics och dashboard.
8. `EPIC-08-evals-och-feedback.md` — Evals och outcome feedback.
9. `EPIC-09-security-och-multitenancy.md` — Auth, tenants, secrets och policy safety.
10. `EPIC-10-cli-sdk-och-integrationer.md` — SDK/CLI och agentintegrationer.
11. `EPIC-11-closed-loop-routing.md` — Closed-loop routing (eval/bandit/A-B/autopilot). **Levererat 2026-07-05.**
12. `EPIC-12-compliance-evidence.md` — Compliance-bevis och audit-export.
13. `EPIC-13-egress-och-pii.md` — Data-egress-kontroll och PII-klassning.
14. `EPIC-14-nyckelprovisioning.md` — Nyckel-provisioning per avdelning.
15. `EPIC-15-ciso-dashboard.md` — CISO-dashboard och rapportering.

## Prioritering

P0 för MVP:

- Epic 1–7.

P1 för beta:

- Epic 8–10.

P0 för CISO-pitchen (strategi 2026-07-05, sprint-09):

- Epic 12–15. Compliance/NIS2 är spjutspetsen; kostnad/CO2 är ROI-beviset.
  Se `00-product/08-positioning-ciso.md`.

P2 för teamprodukt:

- Avancerad RBAC, per-nyckel-budget/rate limit, SIEM-export, ML-baserad PII.
