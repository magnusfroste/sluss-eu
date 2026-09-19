# EPIC-12: Compliance-bevis och audit-export

## Mål

Gör spårbarheten **bevisbar och överlämningsbar**: en revisor/tillsyn ska kunna
få en manipulationssäker logg och en läsbar compliance-rapport som visar vart
data tog vägen, vad som blockerades/maskerades och varför varje beslut fattades.

## Varför

NIS2/cybersäkerhetslagen lägger personligt ansvar på ledningen. Det som skiljer
"vi har loggar" från "vi kan bevisa kontroll" är exportbarhet, integritet
(tamper-evidens) och läsbarhet för icke-tekniker. Detta är kärnan i
CISO-positioneringen (`00-product/08-positioning-ciso.md`).

## Scope

- Hash-kedjad (tamper-evident) audit-export + verifierings-CLI
- Compliance-rapportgenerator (artifakt: policyversion, egress-regler,
  block/mask-räknare, per-avdelnings-spend, modell/leverantörs-inventarie,
  förklarbarhets-sektion)
- CSRD/ESG-siffra (CO2e) i rapporten (P2)

## Out of scope

- Juridiska utfästelser ("compliant") — vi levererar kontroll + bevis.
- Extern SIEM-integration (senare; exporten är förberedelsen).

## Acceptanskriterier

- En export kan verifieras offline (kedjan intakt / manipulation upptäcks).
- Rapporten genereras från befintlig data utan nya hot-path-beroenden.
- Demonstrerbart i lokal devmiljö; test per kritiskt flöde; docs uppdaterade.
