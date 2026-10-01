# ISSUE-117 — UX: förklaring per loggrad, filter, Models och läsbara belopp

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-10-01)

Uppföljning av UX-genomgången (ISSUE-116, punkt 6–9).

## Ändring

- **Request log — varför-rad per anrop.** Klick (eller Enter) på en rad fäller
  ut en förklaring: vart datan gick (lokalt/moln/blockerat, "No provider was
  called"), den lagrade klassningen (task, risk, data class, request-id) och
  vilka regler i *nuvarande* policy som matchar den klassningen
  ("data is pii → only models tagged local"). Prompten lagras aldrig, så
  förklaringen räknas om från klassningen — det står uttryckligen, liksom att
  regler på agentverktyg/prompttermer inte kontrolleras här (dry-run gör det).
- **Request log — snabbfilter** `?show=blocked|sensitive|local|cloud` med
  antal per filter; okänt filter → alla rader. Blockkoden i Task-kolumnen
  kortas med ellips (hela koden står i varför-raden).
- **Models.** local/cloud-bricka per modell (samma taggregel som
  `X-Router-Egress`). Redigeringsformuläret ligger bakom en **Edit**-knapp i en
  utfällbar rad med etiketter, och **Remove** flyttade in där — ingen
  destruktiv knapp ett klick bort i tabellen. Price-kolumnen blev en liten
  "synced/manual" under priset och Test flyttade till Status. Tabellen gick
  från 1950 px till att få plats i 1280 px.
- **Dashboard.** Learning, Shadow routing och Acceptance feedback ligger under
  en hopfälld **Advanced**-sektion (öppnas automatiskt när någon har data);
  de ärliga tomlägena finns kvar.
- **Belopp.** `readableUSD`: under en cent i cent ("0.08¢"), sedan
  `$0.154`, `$12.50` — i stället för sex decimaler.

## Tester

- `TestLogFiltersAndExplanation` — filter, antal, varför-rader, blockerad rad.
- `TestReadableUSD`.
- Befintliga modell-/dashboardtester gröna; skärmdumpar + bredd-mätning av
  logg och Models i 1280 px (roster-läge med redigerbara modeller).
