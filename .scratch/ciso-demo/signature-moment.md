# Signaturögonblicket för CISO-demon (ägarreflektion 2026-07-05)

> Att fånga i minnet — och bygga demon runt.

## Vad som hände (meta, i denna chattsession)

Under den här sessionen körde vi **Fable 5** som default. Vid ett par tillfällen
flaggade Fable 5 en *security notice* i kontexten och **backade** — varpå
**Opus 4.8** tog över (det är den modell vi kör nu). Claude Code bytte modell
**direkt och synligt** — ägaren såg meddelandet i chatten.

Det är exakt produkten, i miniatyr: **något i kontexten flaggas → en modell
backar → en annan, lämpligare modell tar hand om det — och bytet syns.**

## Produktöversättningen

- **Default: en extern molnmodell** körs (billig/snabb/kraftfull) av vanliga skäl.
- **Flagga i kontexten** (PII/personnummer, GDPR-data, säkerhetsklassat innehåll)
  → routa till en **mindre, lokal/on-prem-modell** i stället — för att datan
  inte får lämna huset.
- **Bytet ska vara synligt** i vår egen chat på demo-sidan, precis som ägaren
  såg Claude Code byta modell. "Den här prompten innehöll personuppgifter →
  routad till lokal modell, datan lämnade aldrig EU/huset."

## Vi har redan mekaniken

- **PII-detektion (#77)** flaggar innehållet → höjer sensitivity.
- **Residens-/compliance-taggar (#78)** låter policy kräva taggar. En **lokal
  modell är bara en provider med taggen `local`/`on-prem`/`air-gapped`**.
- Policyn `when: {sensitivity: pii}` + `constraints: {require_provider_tags:
  [local]}` **routar redan** flaggat innehåll till den lokala modellen, och
  blockerar/faller inte tyst tillbaka till moln.

Det som **saknas** är presentationen: att göra bytet **synligt och begripligt**
i demo-chatten (och ett `local`-taggat modellalternativ i demodatan).

## Konsekvens för planen

- **ISSUE-082 (pitchdemo)**: lägg till scenariot "molnmodell default →
  personnummer i prompt → **synligt** byte till lokal modell, med skäl". Detta
  blir demons *känslomässiga* ögonblick (utöver blockering/audit/ROI).
- **Nytt: ISSUE-083** — demo-chat som **visar routningsbytet live**
  (svarsheader `X-Router-Selected-Model` + skäl → en tydlig "routad till
  <modell> för att <skäl>"-banner i chatten). Bygger på befintliga headers.
- Framing i positioneringen: "molnets kraft när det är ofarligt, husets
  trygghet när det inte är det — automatiskt, och du ser exakt när och varför."

Elegant just för att det inte är en ny motor — det är **synliggörandet** av den
egress-kontroll vi redan byggt.
