# ISSUE-042: Implementera secret masking v1

## Labels
- `epic: EPIC-09`
- `priority: P1`
- `type: security`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera secret masking v1 som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Vanliga secrets maskas.
- Masking event loggas.
- Tester med API keys/JWT/DB URLs.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (status-sync)

Implementerat i commit `a9b5bb6` (feat(security): secret masking v1 at the error
boundary). `internal/secrets` (regex-baserad maskning av API-nycklar, bearer-
tokens, JWT, DB-credentials) appliceras på utgående fel-/loggränser via
`server.maskOutbound`. Status-etiketten justerad till `done` för att matcha koden.
