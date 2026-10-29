# ISSUE-137 — Activity och Rollout i designsystemet, klartext, mobilchatten

- **Epic:** EPIC-15 (produktyta)
- **Priority:** P1
- **State:** done (2026-10-29)

## Ändring

- **Activity:** Request log och rapporterna (Incident, Gap, Control) får
  samma sidhuvud med flikarna Request log · Reports. Loggens filter och
  rapporternas nedladdning ligger i en verktygsrad under rubriken.
- **Rollout:** Quick start, Managed devices (f.d. Rollout) och Demo link
  med flikarna Quick start · Managed devices · Connect clients · Demo link.
- **Klartext:** loggen visar uppgift ("Git commit", "Summary"), datatyp
  ("Personal data") och blockeringsorsak ("No allowed model for this data")
  med koden i tooltip; varför-raden säger "Stopped by policy … No model was
  called". Policy-sidans regler läses som meningar ("data is personal data
  or secrets and keys"); rapporter och MCP behåller kodformen.
- **Mobilchatten:** inmatningsfältet får krympa så att Send alltid syns;
  admin-menyn döljs på telefon i chatten.

## Kvar

Sidopanel för modellrader (detaljer + test utan att lämna listan) och
Connect-sidan i konsolramen (den är publik idag). ISSUE-138: ny version och
uppgradering.
