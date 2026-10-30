# ISSUE-142 — Sidopanel för modeller

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Models var en bred tabell där Edit fällde ut en rad med alla fält på en rad.
Status, Test, redigering och "Use this" (ISSUE-134) låg på tre olika ställen.
När DGX-boxen bytte modell till `mimo` krävdes Providers-sidan för att se det.

## Vad

- Klick (eller Enter) på en modellrad öppnar en panel från höger; tabellen
  ligger kvar. Escape, ×-knappen eller klick utanför stänger. Fokus stannar i
  panelen och återgår till raden.
- **Does it answer?** Test med svar i klartext. Vid 400/404 hämtas
  leverantörens egen `/models` automatiskt (annars via knappen), med filter för
  långa listor (OpenRouter) och "Use this" som sparar via `/router/models/slug`.
- **Settings:** samma fält som förut, ett per rad med förklaring. Save går
  genom `POST /router/models` och öppnar samma panel igen (`panel` → `&m=`).
- **Last 24 hours:** antal anrop, fel och kostnad, de senaste anropen
  (uppgift och dataklass, aldrig innehåll) och felen från ErrorRing.
  `GET /router/models/activity?id=`.
- Länk till Request log filtrerad på modellen (`/router/log?model=`).
- `?m=<id>` öppnar panelen direkt; "Needs you" på Overview länkar dit för en
  modell som fallerar.
- Remove ligger längst ner, separat.

## Klart när

- `modelpanel_test.go`: leverantörens lista, aktivitet bara för modellen
  (blockerade och gamla fel räknas inte), en panel per modell, sparande öppnar
  panelen igen, loggfilter per modell.
- Kontroll i webbläsare på desktop och mobil: panelen öppnas och stängs, Test,
  filter, inga JS-fel.
