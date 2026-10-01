# ISSUE-119 — EU-färger: startsida, inloggning och logotyp i produktsajtens palett

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-10-01)

## Bakgrund

Produktsajten www.sluss.eu har fått en EU-profil: djup marinblå bakgrund,
EU-gul accent, IBM Plex Sans/Mono och Instrument Serif i rubriker. Instansens
första två sidor (startsida, inloggning) och logotypen ska kännas som samma
produkt.

## Ändring

- **Palett** (från produktsajtens OKLCH-tema, omräknat till hex): bakgrund
  `#081328`, ytor `#101c34`/`#16243c`, text `#eaeff5`, dämpad `#9aa5b8`,
  accent EU-gul `#fad100` med marinblå text på gula knappar, samma gula
  radiella glöd upptill.
- **Typsnitt:** IBM Plex Sans/Mono och Instrument Serif (h1, med kursiv
  gul avslutning) — **lokalt installerade om de finns, annars systemtypsnitt**.
  Instansen laddar medvetet inga typsnitt från Google: varje sidvisning skulle
  skicka besökarens IP till en tredje part (jfr LG München 2022 om Google
  Fonts), vilket en datasuverän gateway inte ska göra.
- **Logotyp/favicon:** skölden är EU-gul på marinblått (syns även i
  konsolens sidomeny).
- Inloggningssidan: samma palett, gul fokusram och knapp.

## Kvar

Adminkonsolen behåller sin nuvarande blå-gröna palett tills vidare —
egen issue om den ska följa med (status-färger grön/röd behålls oavsett,
de bär betydelse).

## Uppföljning 2026-10-01 — stjärnan och favicon-cachen

- Skölden bär nu **en** marinblå stjärna i stället för routing-grenen —
  läsbar i 16 px. Medvetet en stjärna, inte EU-emblemets krans av tolv:
  emblemet får inte användas så att det ser ut som att EU står bakom en
  produkt.
- Faviconen serverades med 24 h cache på en fast URL, så webbläsare visade
  den gamla gröna efter deployen. URL:en bär nu buildens commit
  (`/favicon.svg?v=<commit>`), så varje ny build tvingar fram ny ikon.
- CI: byggtiden i `sluss_build_info` kom ut tom (`head_commit.timestamp`
  saknas vid vissa merge-händelser); den tas nu från commitens egen tidsstämpel.
