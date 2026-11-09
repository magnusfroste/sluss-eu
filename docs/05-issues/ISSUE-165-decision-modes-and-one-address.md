# ISSUE-165 — Beslutslägen som går att bevisa, och "en adress, stängda dörrar"

`epic: EPIC-15` · `priority: P1` · `state: done` · klar 2026-11-09

## Varför

Två frågor från ägaren:

1. *Om man är ortodox: kan LLM:en stängas av, och beskriver vi regelbaserat +
   LLM tydligt?* Den andra åsikten var av som standard, men dokumentationen sa
   på flera ställen "aldrig en annan AI" utan förbehåll, arkitekturen nämnde
   den inte, och en ortodox organisation kunde inte *visa* att läget var
   rent regelbaserat.
2. *Det många inte förstår:* att Sluss sätts in som en adress i verktygen,
   fast det inte är en proxy, och att verktyg som Entra och Intune stänger
   de andra dörrarna.

## Levererat

- **Bevis för läget:** startbannern (`Decisions`), `GET /version`
  (`decisions`), Version-sidan och en `router.started`-post i auditkedjan vid
  varje start (`decision_mode`, modell). Eftersom kedjan är manipulationssäker
  visar den läget för varje period.
- **Manualsidor:** *One address, closed doors* (först i manualen) och
  *Rules only, or a second opinion*.
- **Precisa formuleringar** i README, manualen, produktbriefen, EVALUATE.md
  och AGENTS.md.
- **Arkitektur:** översikten kallar ingången API-ingång och förklarar att
  Sluss inte är en nätverksproxy; request lifecycle, routing engine och
  säkerhet beskriver andra åsikten, mask and send, svarskontrollen och
  auditankaret.
- **Funktionssvep:** README:s funktionstabell täcker nu alla funktioner.
