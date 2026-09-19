# EPIC-06: Provider execution och fallback

## Mål

Skicka requests till providers, hantera streaming, errors och fallback.

## Varför

Detta epic behövs för att model-router ska kunna leverera låg-latency routing med mätbar kostnads- och kvalitetskontroll.

## Scope

- Adapters
- Streaming normalizer
- Timeouts
- Retries
- Usage/cost capture

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

Se `05-issues/issue-index.md` och filtrera på `epic: EPIC-06`.
