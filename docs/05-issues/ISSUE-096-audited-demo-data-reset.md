# ISSUE-096 — Audited demo-data reset + retention synlig i konsolen

- `epic: EPIC-12`
- `priority: P1`
- `type: product`
- `sprint: backlog`
- `state: done`

## Beslut (2026-07-10)

En delete-knapp i ett bevisverktyg är tveeggat — därför TRE separata mekanismer,
aldrig en generell "radera period"-knapp:

1. **Radering per period = policy, inte knapp** — retention-sweepern (ISSUE-045)
   är den GDPR-försvarbara vägen. Nu SYNLIG i policy-konsolen (dagar +
   prompt-logging-status).
2. **Audited demo-reset** — "Reset demo data" på policy-sidan (admin, confirm):
   tömmer requesthistorik (SQLite), spend/savings, demo-chatsessioner och
   in-memory-räknare (request-logg-ring, comparison/gap, error-ring). Twist:
   **nollställningen skrivs själv i audit-kedjan** (`data.demo_reset`, actor,
   raderade rader) — man kan radera data men aldrig faktumet att man raderade.
3. **Audit-kedjan raderas aldrig selektivt** — rotation/arkivering av hela
   kedjor är rätt livscykel; klipp bryter hashkedjan synligt (featuren).

## Implementation

Reset()-metoder på spend.Tracker, RequestLogTracker, ComparisonTracker,
ErrorRing; history.ResetRequests(); POST /router/data/reset (dashGuard) →
redirect med notice; "Data & retention"-kort på policy-sidan (engelska, per
språkbeslut 2026-07-10). Rör ALDRIG audit-kedja/api_keys/admin_users/roster.

## Framtida

Tenant-radering (offboarda avdelning, GDPR) som separat audited operation.
