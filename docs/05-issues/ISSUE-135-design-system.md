# ISSUE-135 — Designsystem, sex menyval och Overview

- **Epic:** EPIC-15 (produktyta)
- **Priority:** P1
- **State:** done (2026-10-29)

## Bakgrund

Ägaren: kort och listor hade olika bredd, menyn hade femton val, och det
viktiga syntes inte först. Varje sida hade egen CSS och layout. Skissen
(godkänd 2026-10-29) visade riktningen: samma sidhuvud, tre bredder, sex
menyval, "kräver åtgärd" först.

## Beslut: inget SPA

Gemensamma byggstenar i ren CSS + Go-mallar. Ingen byggkedja, en statisk
binär, fungerar air-gappat, går att granska. Enhetligheten kommer från
designsystemet, inte från ett ramverk.

## Ändring

- `design.go`: tokens (färger, avstånd, radier) och komponenter
  (`.ds-content`, `.ds-ph`, `.ds-grid` med `.ds-c12/8/6/4/3`, `.ds-card`,
  `.ds-stat`, `.ds-alert`, `.ds-badge`, `.ds-table`, `.ds-links`,
  `.ds-btn`, `.ds-form`). Skickas med skalets CSS till alla admin-sidor.
- **Menyn:** sex grupper — Overview, Activity, Policy, Models, Access,
  Rollout — plus Open chat, Manual och Sign out längst ner. Den aktiva
  gruppens sidor listas under den (Models → Models, Providers, Risk
  register). Inga URL:er ändrade.
- **Läsbara etiketter** (`labels.go`): dataklasser, uppgiftstyper och
  blockeringskoder visas i klartext; koden finns kvar i tooltip, export
  och audit.
- **Overview** (dashboarden): sidhuvud med primärknapp, **"Needs you"**
  först (modell som fallerat hos sin provider senaste timmen, provider utan
  nyckel, ofärdig setup, enheter med öppen direktväg — varje rad länkar till
  sidan där det åtgärdas), fyra nyckeltal i samma rutnät, flödet och bevisen
  sida vid sida, tabell "By kind of data" i hel bredd. Övriga sektioner
  (rutter, providers, egress, lärande, spend) ligger under "More detail".
  "Blocked fail-closed" heter "Stopped by policy".

## Nästa

ISSUE-136: Models med flikar och sidopanel, Access, Policy i systemet.
ISSUE-137: Activity, Rollout i steg, mobilchatten.
ISSUE-138: "Ny version finns" + uppgraderingsväg (Docker, EasyPanel,
AgentHotel), inspirerat av agenthotel.

## Verifierat

Tester för "needs you" (felande modell, äldre fel faller bort, setup), menyn
(sex grupper, undersidor bara för aktiv grupp) och etiketter. Skärmdumpar
mot lokal instans med mock-provider, desktop och mobil.
