# ISSUE-077: PII-detektor v1 (regelbaserad, fast path-säker)

## Labels
- `epic: EPIC-13`
- `priority: P0`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-05)

## Mål

Upptäck vanlig PII i prompten **innan** den lämnar huset och höj
`sensitivity`/risk så policyn kan blockera, tvinga EU-residens (ISSUE-078)
eller eskalera tier. Regelbaserad (regexp + checksummor) — ingen LLM, ingen
nätverk, väl inom feature-extractorns p95 < 20 ms.

## Detektorer v1

- **Svenskt personnummer** (ÅÅMMDD-XXXX / ÅÅÅÅMMDD-XXXX, Luhn + datumvalidering)
- **E-postadress**
- **Telefonnummer** (svenskt + E.164-heuristik)
- **IBAN** (mod-97)
- **Kortnummer** (13–19 siffror, Luhn)

Varje träff ger: typ + antal (aldrig själva värdet i logg/audit — bara
`pii:personnummer x2`).

## Design

- Nytt paket `internal/pii` med `Detect(text) []Finding`; anropas från
  feature-extractorn och sätter `Sensitivity` (minst `personal`/motsvarande
  befintlig nivå) + signal i JobDescriptor (`pii_types`).
- Policy kan matcha på sensitivity som idag; decision reasons får
  "pii detected: personnummer".
- Auditpost vid PII-träff som ledde till block/eskalering (typ + antal, ej värde).
- Falskt-positivt-hållning: detektorn **eskalerar**; hårt block är alltid
  policyns beslut, inte detektorns.

## Acceptanskriterier

- Tabellstyrda tester per detektor: träffar, icke-träffar (Luhn-ogiltiga,
  datumogiltiga), svenska specialfall (samordningsnummer +60).
- Bench: Detect på 10 kB väl inom extractorns p95 < 20 ms-budget.
  (Uppmätt: ~1,2 ms/10 kB på delad 2,1 GHz vCPU, 14 allocs — fem RE2-pass;
  1 ms-målet från utkastet justerat till uppmätt verklighet.)
- Golden policy-case: prompt med personnummer → eskalerad routing/block.
- Inga PII-värden i loggar, audit eller event-kön — endast typ + antal.
