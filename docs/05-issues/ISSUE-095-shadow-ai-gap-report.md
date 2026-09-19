# ISSUE-095 — Shadow AI gap-rapport (monitor mode) — CISO-kroken

- `epic: EPIC-12`
- `priority: P0`
- `type: product`
- `sprint: 09`
- `state: done`

## Idé (säljkroken)

CISO:ns verkliga problem är inte att sätta policy — det är att hen inte vet vad
som redan läcker, och ingen vågar slå på blockering dag 1. Monitor mode löser
båda: kör ett compliance-paket som **skuggpolicy** (`ROUTER_SHADOW_POLICY_PATH=
builtin:nis2-baseline`) — inget blockeras, inget ändras — och låt
`/router/gap-report` räkna vad paketet SKULLE ha gjort med känsliga prompts.
Efter 14 dagar: "N prompts med personuppgifter gick till US-moln; under paketet
hade 100 % stannat i huset." Konvertering = flytta paketet till
`ROUTER_POLICY_PATH`. Land-and-expand som brandväggars TAP-läge.

## Implementation

- `eventlog.ComparisonTracker`: `Sensitivity` på ComparisonRecord + exakta
  `GapSummary`-räknare (sensitive total/pii/secrets, route-changed,
  shadow-blocked) sedan omstart — oberoende av recent-ringen (50).
- `internal/server/gapreport.go`: `GET /router/gap-report` (admin-gated;
  markdown default, `?format=json`). Monitor mode AV → recept; PÅ → siffrorna,
  procentraden, exempeltabell (task/känslighet/live-modell(egress) → paketets
  utfall — **aldrig** prompttext), konverteringssteget, credibility-raden.
- Återanvänder skuggpolicy-maskineriet (ISSUE-055) och regime-profilen.

## Ärlighet

Räknare exakta sedan senaste omstart (in-memory), ingen prompttext lagras/visas,
alltid "kontrollrapport, inte juridiskt intyg".

## Kvar (uppföljare)

ISSUE-096-kandidat: incident-readiness — larm (webhook/mejl) vid blockerade
försök + 24/72h-evidensexport ur audit-kedjan.
