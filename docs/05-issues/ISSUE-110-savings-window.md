# ISSUE-110 — Tidsfönster på `savings_report` (MCP)

- **Epic:** EPIC-12 (compliance evidence)
- **Priority:** P2
- **State:** done (2026-07-25)
- **Bakgrund:** Odysseus-rekognosceringen 2026-07-25 (agent-driven CISO-morgonbrief via MCP).

## Problem

`savings_report` returnerade alltid kumulativa (all-time) aggregat medan
`incident_report` redan hade riktiga fönster (24h/72h/7d). En agent som ska
skriva en *daglig* brief kunde alltså rapportera "vad hände igår" men bara
"vad har vi sparat sedan tidernas begynnelse" — siffran ändrar aldrig form och
säger inget om dygnet.

## Ändring

- **`history.Store.ByModelSince(t)`** — samma per-modell-aggregering som
  `ByModel()` men tidsfiltrerad (nolltid = ingen nedre gräns).
- **`savings_report` tar `window`**: `24h`, `72h`, `7d`, `30d` — samma
  vokabulär som `incident_report`, så en brief kan fråga båda verktygen om
  *samma* period. Utelämnat (eller `all`) = kumulativt som förr
  (bakåtkompatibelt).
- Fönstrat läge räknar om **både** savings- och green-counterfactualen över
  bara den perioden, med samma deterministiska baseline (ISSUE-107), och
  redovisar `window_from`.
- **Fail-safe:** okänt fönster → faller tillbaka till all-time (inte ett tyst
  fel). Saknas den durabla loggen returneras all-time med `window_note` som
  förklarar varför — hellre en sann kumulativ siffra än en påhittad dygnssiffra.

## Tester

`TestSavingsReportWindow`: rader inom och utanför fönstret; 24h utesluter den
gamla raden (2 av 3 requests), kostnad och premium-baseline räknas om korrekt
för perioden, `window_from` sätts, baseline-modellen följer med, och okänt
fönster faller tillbaka till all-time.

## Not

Blockerade requests räknas inte i savings (de har ingen modell/kostnad) — de
finns i `incident_report`, som är rätt verktyg för dem. En agent som skriver
morgonbriefen bör hämta båda för samma fönster.
