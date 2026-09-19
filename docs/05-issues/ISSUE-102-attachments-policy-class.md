# ISSUE-102 — Bilagor som policyklass (array-content-stöd)

- category: enhancement
- state: done
- epic: EPIC-13
- priority: P1
- type: backend|security
- sprint: 09
- klar: 2026-07-18

## Bakgrund

Pitch-invändning 2026-07-18: "vad händer om någon bifogar ett dokument — då är
inte prompten bäraren?" Teknisk verifiering visade: (1) inlinad dokumenttext
klassas redan perfekt (personnummer hittades mitt i ett "avtal"), men (2)
array-form content (vision-format, som vissa SDK:er skickar även för ren text)
avvisades med 400 vid decode — fail-closed men oartigt, och bilder kunde inte
policy-klassas alls.

## Levererat

- **`openai.Message` accepterar båda content-formerna**: sträng (oförändrad
  fast path) och array av parts. Textdelar konkateneras in i `Content` — så
  klassificeraren, maskningen och loggen ser HELA dokumenttexten; original-
  arrayen bevaras orörd (`ContentParts`) och re-emitteras till providern
  (vision-payload når modellen exakt som klienten skickade den).
- **Icke-textdelar (bild etc.) sätter `NonTextParts`** → klassificeraren höjer
  `requires_vision` + keywords `vision`/`attachment` — oläsbart innehåll blir
  en egen policyklass i stället för en blind fläck.
- **NIS2-paketet regel 4c** (`pv_nis2_baseline_2026_07b`):
  `requires_vision: true` → require `local` — det som inte kan läsas hålls i
  huset eller fail-closas. "What cannot be classified is confined categorically."
- Kandidatfiltret kräver redan vision-kapabel modell, så bild-requests
  begränsas dubbelt: capability + policy.

## Verifiering

Round-trip-tester (sträng oförändrad; array text-only → konkatenerad +
original-array till providern; bilddel → flagga + payload bevarad; ogiltiga
former → rent fel; tool-fält överlever). Pipeline-tester: dokument-PII klassas
(`personnummer` i bilagan), bilddel → `requires_vision`. Live: array-content
med personnummer → 200, LOKAL, `pii:personnummer` (förut 400); bild-request
under nya regeln → fail-closed `residency_no_compliant_provider`.

## Pitch-svaret (tre lager)

1. I API:et blir bilagan prompt — vi klassar varje byte av innehållet.
2. Det vi inte kan läsa gissar vi aldrig om — det begränsas kategoriskt
   (lokal-only eller block, fail-closed).
3. Djup filinspektion (OCR/EDM över alla appar) är SSE-klassens DLP — ärligt
   loggat gap (`00-product/11 §4b`), inte vårt anspråk.
