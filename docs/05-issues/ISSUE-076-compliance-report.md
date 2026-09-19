# ISSUE-076: Compliance-rapportgenerator (artefakt för revisor/ledning)

## Labels
- `epic: EPIC-12`
- `priority: P0`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-05)

## Mål

Generera en **läsbar compliance-rapport** (markdown-artefakt; HTML/PDF senare)
som en CISO kan lämna vidare: vilken kontroll som är aktiv, vad som hänt, och
varför besluten är förklarbara. Rapporten är "beviset" i pitchen.

## Innehåll (sektioner)

1. **Kontrollstatus** — aktiv policyversion + registry-version, egress-regler
   (allow/deny provider+modell, residens-villkor från ISSUE-078), aktiva
   incident-spakar (conservative mode, circuit-status, budgettak).
2. **Händelser (period)** — antal requests, blockerade (per block-orsak),
   maskerade secrets/PII-träffar, budget-events.
3. **Per avdelning** — requests/spend/blockeringar per tenant/projekt.
4. **Förklarbarhet** — deterministisk routing utan LLM i beslutet; exempel på
   decision reasons; hänvisning till audit-exporten (ISSUE-075).
5. **ROI** — savings vs all-premium + CO2e (green receipt). Tydligt märkt
   estimat.

## Design

- `cmd/compliance-report` (eller underkommando) + `GET /router/compliance/report`
  (admin) — båda bygger på samma builder i `internal/report`.
- All data hämtas från befintliga källor: history/spend, audit, policy cache,
  health/circuit, dashboard-builders. **Inga nya hot-path-beroenden.**
- Språk: revisorspråk, inte modell-slugs först. Inga juridiska utfästelser —
  rubriken är "kontrollrapport", inte "compliance-intyg".

## Acceptanskriterier

- Rapport genereras i lokal devmiljö med demo-data; tom-tillstånd är ärliga.
- Golden-test på rapportstruktur (sektioner + nyckelfält) med fixad indata.
- Dokumentation: docs/compliance-report.md med exempel.
