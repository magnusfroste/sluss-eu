# ISSUE-083: Synligt routningsbyte i demo-chatten (moln → lokal)

## Labels
- `epic: EPIC-15`
- `priority: P1`
- `type: frontend`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Bakgrund (signaturögonblicket)

Under utvecklingssessionen körde vi Fable 5 som default; när innehåll flaggades
backade den och Opus 4.8 tog över — **och bytet syntes i chatten**. Det är
produkten i miniatyr, och ägarens känslomässiga "aha". Se
`.scratch/ciso-demo/signature-moment.md`.

Produktöversättning: **molnmodell default** (billig/snabb) → **flagga i
kontexten** (PII/personnummer/känsligt) → routa till en **lokal/on-prem-modell**
så datan inte lämnar huset — **och bytet visas** med skäl.

Mekaniken finns redan: PII-detektion (#77) höjer sensitivity; residens-taggar
(#78) låter policy kräva `require_provider_tags: [local]`. En lokal modell är
bara en provider med taggen `local`. **Det som saknas är att synliggöra bytet.**

## Mål

Gör routningsbytet **synligt och begripligt** i chatten (`/chat`): när en modell
som inte är den "förvalda" väljs, visa en tydlig banner —
*"🔀 Routad till **<modell>** — <skäl> (t.ex. personuppgifter upptäckta → måste
stanna i huset)"*. Bygger på befintliga svarsheaders, ingen ny routinglogik.

## Design

- Chatten läser redan svaret; exponera routningsmetadata till klienten via de
  headers som redan sätts: `X-Router-Selected-Model`, `X-Router-Route-Class`,
  och lägg vid behov till en kort `X-Router-Route-Reason` (topp-skäl ur
  `decision_reasons`, redan beräknat — inga prompt-/PII-värden).
- Banner ovanför svaret när `selected_model` ≠ default-/förväntad modell, eller
  när route-class/skäl indikerar en egress-styrd routning (t.ex. sensitivity=pii
  + residenskrav). Grön/neutral ton, en mening, med skälet.
- Ingen känslig data i bannern — endast typ/skäl ("personuppgifter upptäckta"),
  aldrig värdet. Följer maskeringsprincipen.

## Icke-mål

- Ingen ny modell/adapter för "lokal" — det är demodata (en provider taggad
  `local`). Riktig on-prem-adapter är senare.
- Ingen ändring av fast path / routingbeslut — detta är presentation.

## Acceptanskriterier

- I demo-chatten: en prompt med (syntetiskt) personnummer mot en policy som
  kräver `local` → svaret kommer från den lokala modellen och bannern förklarar
  bytet med skäl.
- En vanlig prompt → ingen banner (eller neutral "moln, standard").
- Handler-/rendering-test för bannerns villkor; inga PII-värden i utdata.
