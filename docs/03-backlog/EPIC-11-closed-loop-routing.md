# EPIC-11: Closed-loop routing improvement

> Formaliserar det epic-nummer som redan refereras av ISSUE-062–069 m.fl.
> Status: **i allt väsentligt levererat** (2026-07-05).

## Mål

Sluta loopen: produktionssignal → mätning → rekommendation → säker tillämpning.
Routing ska förbättras av verklig utfallsdata, inte statiska antaganden.

## Levererat

- Eval-driven scoring (uppmätta pass rates i scoringen) — PR #52
- Provider circuit breaker (cooldown + half-open probe) — PR #53
- Council/Fusion-läge för högriskklasser — PR #82
- Automatisk policyrekommendation från cost/quality-frontieren — PR #83
- Live policy-A/B med deterministisk trafiksplitt — PR #84
- Bandit-routing (UCB1, deterministisk) med warm start från evals — PR #85
- Autopilot med guardrails (granskningsbar auto-godkänd policy) — PR #86

## Kvar (P2, vid behov)

- Outcome-baserad bandit-reward (acceptans, inte bara attempt-success)
- Per-arm/bandit-panel i dashboard (flyttad till EPIC-15)

## Acceptanskriterier

- Uppfyllda: varje del är env-gated/av som default, deterministisk på fast
  path, testad och dokumenterad.
