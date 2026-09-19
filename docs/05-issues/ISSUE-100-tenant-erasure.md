# ISSUE-100 — Auditerad tenant-erasure (GDPR art. 17)

- category: enhancement
- state: done
- epic: EPIC-12
- priority: P1
- type: backend|security
- sprint: 09
- klar: 2026-07-12

## Bakgrund

Sista GDPR-luckan i evidenskedjan: en kund/tenant begär radering. Retention
raderar per tid; detta raderar per SUBJEKT — med samma evidenshållning som
demo-reseten (ISSUE-096): audit-kedjan raderas aldrig selektivt, och raderingen
i sig loggas där.

## Levererat

- `POST /router/data/erase-tenant` (admin): raderar EN tenants request-rader ur
  durabla loggen (`history.EraseTenant`) + spend-attribution i minnet
  (`spend.Tracker.EraseTenant`); valfritt återkallas tenantens API-nycklar i
  samma handling (så subjektet inte tyst ackumulerar ny data minuten efter).
- Auditeras som `data.tenant_erasure` med aktör, tenant och räknare
  (rows_deleted / keys_revoked) — "you can delete data, but never the fact
  that you deleted it".
- Formulär i policy-konsolens Data & retention-kort (tenant-id + checkbox för
  nyckelåterkallelse, confirm-dialog).
- Modell-aggregat (ej tenant-attribuerade) och andra tenants data orörda.

## Verifiering

Unit: selektiv radering (andra tenants rader/nycklar/spend kvar), audit-detalj
med räknare, tomt tenant-id avvisas, formulär i konsolen. Live: riktig trafik →
erase tn_demo → "3 request rows deleted" + audit-post med aktör och räknare.
