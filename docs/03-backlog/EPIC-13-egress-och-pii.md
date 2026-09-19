# EPIC-13: Data-egress-kontroll och PII-klassning

## Mål

Stärk grinden **innan** data lämnar huset: upptäck PII/känslig data vassare och
gör dataresidens/leverantörs-compliance till förstklassiga policyvillkor
("den här dataklassen får aldrig lämna EU / får bara gå till DPA-godkänd
leverantör").

## Varför

Egress-kontroll är CISO-wedgens hjärta. Vi har policy-blocket och maskingen;
gapen är (1) detekteringskvalitet för PII och (2) residens/compliance-metadata
som policyn kan uttrycka sig mot.

## Scope

- PII-detektor v1: regelbaserad (ingen LLM, fast path-budget): svenskt
  personnummer, e-post, telefon, IBAN, kortnummer (Luhn) → höjer sensitivity
  + auditpost + maskering där det är säkert
- Compliance-taggar i rostern per provider/modell (region/residens, DPA,
  certifieringar) — data, inte kod
- Policyvillkor mot taggarna: `require_provider_tags` / `deny_provider_tags`
- Egress-beslutet syns i decision reasons + audit ("blockerad: eu-only")

## Out of scope

- ML-baserad PII-NER (senare; regelbaserat först — deterministiskt, snabbt,
  förklarbart)
- Innehållsomskrivning utöver befintlig maskering

## Acceptanskriterier

- Fast path förblir LLM-fri och inom latensbudget (detektorn är ren regexp/
  checksum).
- Falskt-positiv-nivå hanterbar: eskalering hellre än hårt block där osäkert.
- Golden cases i policy-testerna för residens-regler; docs uppdaterade.
