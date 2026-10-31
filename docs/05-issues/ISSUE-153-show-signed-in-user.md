# ISSUE-153 — Visa vem som är inloggad

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Ägaren loggade in som `demo@sluss.eu` och kunde inte se vilket konto det var.
Menyns fot visade hårdkodat "admin". `demo@sluss.eu` är inget konto: med
nödlösenordet (`ROUTER_DASHBOARD_PASSWORD`) loggar man in som admin under det
namn man skriver, och sessionen/auditloggen sparar det som
`demo@sluss.eu (break-glass)`.

## Vad

- Menyns fot visar den inloggade (`GET /router/whoami`); en nödinloggning
  visas i gult som `namn · break-glass` med förklaring i tooltip.
- Console users markerar den egna raden med "you" och förklarar en
  nödinloggning: namnet är inget konto, skapa ett namngivet konto för vardag.
- Knappen Inaktivera/Aktivera på engelska (Disable/Enable) som resten av UI.
