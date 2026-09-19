# PII-detektering (ISSUE-077)

Feature-extractorn kör en regelbaserad PII-detektor (`internal/pii`) på varje
prompt **innan** något lämnar huset. Ingen LLM, inget nätverk — regexp +
checksummor, deterministiskt och inom fast path-budgeten (~1,2 ms per 10 kB;
skanningen är cappad till 64 KiB per meddelande).

## Detektorer v1

| Typ | Validering |
|---|---|
| `personnummer` | ÅÅMMDD/ÅÅÅÅMMDD + Luhn + datumregler (samordningsnummer +60) |
| `email` | pragmatisk RFC-regex |
| `phone` | E.164 (`+` och 8–15 siffror) eller svenskt format (ledande 0) |
| `iban` | ISO 13616 mod-97 + längd |
| `card` | 13–19 siffror + Luhn |

## Vad en träff gör

Detektorn är en **eskaleringssignal, aldrig en dom**:

1. `sensitivity` höjs till `pii` och risken till minst `medium`
   (befintlig klassificeringsväg).
2. `JobDescriptor.PIITypes` sätts (t.ex. `[personnummer]`) — typ + antal,
   **aldrig värdena** — så policy kan blockera, tvinga residens (ISSUE-078)
   eller eskalera tier.
3. Klassificeringsskäl per typ: `pii_detected:personnummer` (förklarbarhet).
4. Blockeras requesten får auditposten `pii_types` (typer, inte värden).

Hårt block är alltid **policyns** beslut — detektorn höjer bara signalen, så
falskt positiva eskalerar i värsta fall routingen i stället för att stoppa
legitim trafik.

## Integritetsprincip

Inga matchade värden lämnar detektorn: findings är `{typ, antal}`. Inget PII
hamnar i loggar, audit, event-kön eller dashboards.
