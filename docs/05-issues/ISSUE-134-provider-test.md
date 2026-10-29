# ISSUE-134 — Test per provider, och byt modell till det providern faktiskt serverar

- **Epic:** EPIC-15 (produktyta)
- **Priority:** P1
- **State:** done (2026-10-29)

## Bakgrund

Ägaren labbar på DGX och bytte modell till *mimo*. Sluss hade kvar
`glm-5.3-flash`, providern svarade `model_unavailable`, och chatten gav ett
fel. Providers-sidan visade "health 100%", ett passivt mått på trafiken, som
såg ut som en live-status. Ägaren: "det är inget jag litar på".

## Ändring

- **Test per provider** på Providers-sidan: frågar providerns `GET /models`
  vad den serverar och skickar en 1-token-förfrågan till varje modell som
  routas dit (samma anrop som ISSUE-098:s rad-test). Visar ✓/✗ per modell,
  latens, diagnos, och **"registered here but not served there"** när en
  modell bytts ut eller tagits ner på servern.
- **"Use this for <modell>"** bredvid varje serverad modell (när en modell
  routas till providern): byter modellens slug, men bara till en slug
  providern själv listar, så ett skrivfel kan inte sparas. Ändringen laddas
  live och auditeras som en vanlig modellsparning. Testet körs om direkt.
- "health" heter nu **"traffic health"** med en förklaring: andel lyckade
  anrop i senaste trafiken, passivt.

## Verifierat

Tester mot en fejk-provider som serverar `mimo`: testet flaggar
`glm-5.3-flash` som saknad och modellen som ✗; slug som inte serveras
avvisas (400); "Use this" sparar `mimo` och testet går sedan grönt.
