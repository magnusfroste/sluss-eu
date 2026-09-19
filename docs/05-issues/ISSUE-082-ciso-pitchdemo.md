# ISSUE-082: CISO-pitchdemo — scenario, dataset och körschema

## Labels
- `epic: EPIC-15`
- `priority: P1`
- `type: task`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Mål

Ett reproducerbart pitch-scenario (script + demodata + körschema) som visar
hela värdekedjan på < 10 minuter för en CISO. Demon ÄR leverabeln för
sprint-09 — allt annat i sprinten är dess byggstenar.

## Scenario (körschema)

1. **Setup** (före mötet): router uppe (mock eller OpenRouter), två
   avdelningsnycklar ("ekonomi", "utveckling"), policy med EU-residens-krav
   för personuppgifter, lite förkörd trafik så dashboarden lever.
2. **Signaturögonblicket (moln → lokal)**: default är en molnmodell. Skicka en
   prompt med svenskt personnummer → PII flaggas → policy kräver `local` →
   svaret kommer från den **lokala** modellen och chatten visar **synligt**
   bytet: *"🔀 Routad till lokal modell — personuppgifter upptäckta, datan
   lämnade aldrig huset"* (ISSUE-083). Detta är demons känslomässiga kärna.
2b. **Läckageförsöket (hårt block)**: samma prompt mot en policy utan lokal
   modell → **blockeras** med skäl + decision reasons ("pii detected", "saknar
   tag eu-resident"). Visar att grinden faller stängt, inte tyst till moln.
3. **Beviset**: visa auditposten på dashboarden → exportera audit-kedjan →
   kör `verify` live → manipulera en rad → kör `verify` igen → **brott
   upptäcks**. ("Det här kan ni lämna till tillsynen.")
4. **Uppföljningen**: Egress & Compliance-vyn per avdelning; generera
   kontrollrapporten (ISSUE-076).
5. **Avslutet (ROI)**: savings-heron + CO2 — "kontrollen betalar sig själv".

## Leverabler

- `scripts/demo-ciso.sh` — startar, seedar nycklar/policy/trafik (bygger på
  smoke-harnessens mönster; körbar mot mock utan credentials).
- `examples/demo-ciso/` — policyfil, demo-prompts (inkl. syntetiskt
  personnummer — ALDRIG äkta data), README med körschemat ovan + talepunkter.
- Talepunkterna följer positioneringen: kontroll + bevis; inga
  compliance-utfästelser; kostnad/CO2 som avslut, inte rubrik.

## Acceptanskriterier

- `scripts/demo-ciso.sh` kör hela scenariot end-to-end mot mock, grönt i CI
  (som `make smoke`).
- Demon fungerar även mot OpenRouter med nyckel satt (manuell körning).
- All demodata är syntetisk; README:n markerar detta uttryckligen.
