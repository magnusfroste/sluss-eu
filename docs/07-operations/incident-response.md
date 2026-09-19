# Incident response

## Severity

| Severity | Exempel |
|---|---|
| SEV1 | Routern nere, alla requests påverkas |
| SEV2 | Major provider fallback fungerar inte |
| SEV3 | Felrouting för specifik taskklass |
| SEV4 | Dashboard eller rapport felaktig |

## Incidentprocess

1. Triage.
2. Utse incident lead.
3. Stoppa blödning: fallback, conservative mode eller rollback.
4. Kommunicera status.
5. Åtgärda root cause.
6. Skriv postmortem.
7. Lägg till regressionstest.

## Conservative mode

Global conservative mode ska kunna:

- Routea okända tasks till balanced eller premium.
- Blockera externa providers för känsliga projekt.
- Sänka användning av degraded providers.

## Postmortemkrav

- Vad hände?
- Vilka tenants påverkades?
- Vilka policies/models påverkades?
- Hur upptäcktes incidenten?
- Varför fungerade/fungerade inte fallback?
- Vilka tester saknades?
