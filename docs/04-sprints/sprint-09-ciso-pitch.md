# Sprint 09: CISO pitch MVP

## Mål

En **fungerande produkt att pitcha för en CISO**: läckageförsök blockeras live,
revisionsloggen exporteras och verifieras, egress-kartan visas per avdelning,
och besparingen (kr + CO2) landar som avslut. Strategi: compliance/NIS2 som
spjutspets, kostnad+CO2 som ROI-bevis (DECISION_LOG 2026-07-05,
`00-product/08-positioning-ciso.md`).

Allt byggs genom **återanvändning**: policy engine, audit-sink, masking,
tenants/budget, roster i SQLite, dashboard, green receipt.

## Issues (pitch-kritisk väg)

| Issue | Epic | Vad |
|---|---|---|
| ISSUE-075 | EPIC-12 | Tamper-evident audit-export (hash-kedja) + verify-CLI |
| ISSUE-076 | EPIC-12 | Compliance-rapportgenerator (artefakt för revisor) |
| ISSUE-077 | EPIC-13 | PII-detektor v1 (regelbaserad, fast path-säker) |
| ISSUE-078 | EPIC-13 | Residens-/compliance-taggar + policyvillkor |
| ISSUE-079 | EPIC-14 | DB-backade API-nycklar v1 (mint/revoke, hash i vila) |
| ISSUE-080 | EPIC-14 | Keys-adminsida + audit av mint/revoke |
| ISSUE-081 | EPIC-15 | Dashboard: "Egress & Compliance" + "Learning & routing" |
| ISSUE-082 | EPIC-15 | CISO-pitchdemo: scenario, dataset och körschema |

Rekommenderad ordning: 077+078 (egress-grinden) → 075+076 (beviset) →
079+080 (avdelningar) → 081 (vyn) → 082 (demon knyter ihop).

## Definition of Done

- Demo-scenariot i ISSUE-082 går att köra end-to-end i lokal devmiljö
  (mock eller OpenRouter) utan handpåläggning.
- Export verifieras offline; manipulation upptäcks demonstrerbart.
- Fast path förblir LLM-fri och inom latensbudget (p95 < 100 ms routing).
- Två demo-avdelningar (nycklar) syns separat i uppföljningen.

## Demo

Pitch-körningen: (1) skicka prompt med personnummer mot icke-EU-modell →
**blockeras**, skäl i svaret; (2) visa auditposten + exportera och verifiera
loggen; (3) visa egress/避spend per avdelning; (4) avsluta med savings + CO2.

## Risker att följa upp

- PII falskt-positiva (eskalera hellre än blockera hårt där osäkert).
- Nyckel-provisioning får inte sakta ner auth-pathen (cache framför DB).
- Rapporten måste tala revisorspråk — testa på icke-tekniker.
- Scope creep: inga juridiska utfästelser, ingen SIEM-integration i denna sprint.

## Efter sprinten (kandidater, ej planerade)

Per-nyckel-budget/rate limit, CSRD-export, ML-PII (NER), SIEM-export,
retention-policy per dataklass.
