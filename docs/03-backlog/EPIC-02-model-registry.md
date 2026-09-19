# EPIC-02: Model registry

## Mål

Skapa registry för modeller, providers, capabilities, kostnader och latencyprofiler.

## Varför

Detta epic behövs för att model-router ska kunna leverera låg-latency routing med mätbar kostnads- och kvalitetskontroll.

## Scope

- Model metadata
- Provider metadata
- Registry versionering
- In-memory cache
- Admin reload

## Out of scope

- Fullständig enterprisefunktionalitet om inte uttryckligen nämnt.
- Automatisk ML-optimering.
- Provider-specifika specialfall utanför MVP-kontraktet.

## Acceptanskriterier

- Funktionerna kan demonstreras med lokal devmiljö.
- Minst ett automatiserat test täcker varje kritiskt flöde.
- Metrics och logs finns för ny funktionalitet.
- Dokumentation uppdateras.

## Risker

- Scope creep.
- Latencybudget överskrids.
- Providerformat visar sig mer komplext än antaget.

## Relaterade issues

Se `05-issues/issue-index.md` och filtrera på `epic: EPIC-02`.
