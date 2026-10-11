# ISSUE-124 — Quick start: två modeller och en regel, med live-status

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-10-11)

## Bakgrund

Jämförelsen med LiteLLM:s "pick 4 models and go": första mötet med en
nyinstallerad Sluss var en tom konsol med åtta sidor. Den som laddar ner
produkten ska komma till "personuppgifter stannar lokalt, resten går till ett
billigt moln" på några minuter.

## Ändring

- **`/router/setup` — Quick start**, först under Control i menyn. Fyra steg,
  vart och ett med *live*-status räknad från den körande gatewayn (aldrig en
  sparad bock):
  1. **En molnmodell** — routbara modeller utan local-tagg.
  2. **En lokal modell** — ett formulär (namn, bas-URL, modellnamn, namnet på
     nyckelns env-variabel) skapar provider + modell i ett steg: taggad
     `local, on-prem`, tier balanced, pris 1.0/4.0 så att den bara vinner när
     policyn kräver lokalt. Går live direkt (ISSUE-115). Ett nyckel*värde* i
     stället för ett env-namn avvisas.
  3. **Personuppgifter stannar lokalt** — utvärderar den *aktiva* policyn mot
     en personuppgiftsprompt och säger vad som händer. Knappen "Keep personal
     data and secrets local" lägger till konsolregeln (pii + secrets_possible →
     local) och aktiverar den, auditerat — men visas och verkar bara när
     policyn inte redan skyddar, så den ersätter aldrig ett compliance-paket.
  4. **Se det fungera** — dry-run av ett personnummer (→ lokalt) och ett
     commit-meddelande (→ moln); steget klart vid första riktiga anropet.
- **Dashboarden** visar "Finish setting up — N of 4 steps done" tills alla
  steg är klara.
- **Startsidan** pekar på Quick start i Getting started.

## Tester

`TestQuickStartFlow` (färskt läge, avvisat nyckelvärde, lokal provider +
modell med taggar och pris, skyddsregel aktiveras, andra tryck är no-op,
sidan renderar), `TestSlugID`. End-to-end på en ny instans: lokal modell
live utan omstart, regel aktiverad, dry-run lokalt/moln.
