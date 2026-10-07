# ISSUE-123 — Dashboard: flödesbild och 7-dagars tidslinje

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-10-07)

## Bakgrund

UX-svepet 2026-10-02: dashboarden visade bara totaler (en stapel + tabell).
Ingen tidsdimension, och "vart tog datan vägen" var svårare att läsa på scen
än det behöver vara.

## Ändring

- **Flödesbild** överst i "Where your data went": alla klassade prompts till
  vänster, band till *Stayed in the house / Went to the cloud / Blocked
  fail-closed* (och *Unknown* för äldre rader) till höger, bandbredd ∝ antal,
  med antal och procent som direktetikett. Ersätter den platta stapeln;
  tabellen per datakategori finns kvar under.
- **Last 7 days**: staplade kolumner per UTC-dag (lokalt i botten, moln,
  blockerat överst), tomma dagar ifyllda, dagens total som enda
  direktetikett, hovring visar exakta värden, och **Show as table** ger samma
  siffror som tabell. Ny `history.EgressRowsSince` aggregerar per dag;
  äldre rader klassas med samma regel som resten (modell → provider →
  unknown).
- **Ritat på servern som inline-SVG** — inget diagrambibliotek, inga externa
  resurser, fungerar utan JavaScript; ett litet skript lägger till tooltip och
  startar tidslinjen på senaste dagen i telefonbredd (diagrammet scrollar i
  sin ruta, sidan flödar inte över).
- **Palett validerad** mot panelen `#101c34` (dataviz-validatorn: ljushetsband,
  kromaminimum, CVD-separation, normalseende, kontrast): lokalt `#1fb054`
  (något mörkare grön — den gamla föll utanför ljushetsbandet), moln
  `#3b82f6`, blockerat `#ef4444`. Identitet bärs aldrig av färg ensam:
  legend, direktetiketter, tooltip och tabellvy.
- **Ärlig negativ besparing**: blir kostnaden högre än all-premium-baslinjen
  står det nu "X% more expensive" i en bärnstensfärgad ruta med en hint om
  modellpriserna — i stället för "-233% cheaper" i grönt.

## Tester

`TestNiceMax`, `TestTimelineAndFlowCharts` (dagsfönster, ifyllda dagar,
utanför fönstret exkluderas, tooltip-text, etiketter, tomt läge).
Skärmdumpar desktop + 390 px mot en vecka syntetisk historik.
