# ISSUE-120 — Riskregistret får en egen sida; Providers-formuläret renderas rätt

- **Epic:** EPIC-12 (compliance surface)
- **Priority:** P2
- **State:** done (2026-10-01)

## Bakgrund

Ägarens fynd: formuläret på Providers renderade inte rätt, och riskregistret
låg "mellan listan och formuläret". En provider-anslutning (endpoint +
nyckel) är drift; riskregistret är leverantörsstyrning — vad CISO:n intygar
om varje leverantör (DPA, certifieringar, residens) och vad policyn kräver.
Olika fråga, olika ägare → egna sidor.

## Ändring

- **`/router/risk` — Risk register** (egen post i sidomenyn under Control,
  efter Providers): taggmatrisen per provider, vilka taggar aktiv policy
  *kräver*, och fail-closed-varningen när en krävd tagg saknas. Spara per rad
  går som förut via `POST /router/providers/tags` (auditerat) men landar nu
  tillbaka på registret.
- **Providers** visar bara anslutningarna och lägg-till/redigera-formuläret.
  Fail-closed-varningen finns kvar där (den är för viktig att gömma) med länk
  till registret, och introtexten pekar dit.
- **Renderingsfel:** formulärfälten var `width:100%` + padding utan
  `box-sizing:border-box` och stack ut till höger — rättat. Registrets tabell
  var 986 px i 941 px (Save-knappen klipptes) — får nu plats. Edit/Remove i
  provider-korten ligger i en egen grupp och bryts aldrig isär.

## Tester

`TestRiskRegisterGapWarningAndTagUpdate` testar registret på `/router/risk`,
att taggsparning återvänder dit, och att Providers har kvar varningen men
inte matrisen. Skärmdumpar + bredd-mätning av båda sidorna i 1280 px.
