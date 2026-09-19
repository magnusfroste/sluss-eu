# ISSUE-057: Implementera RBAC skeleton

## Labels
- `epic: EPIC-09`
- `priority: P2`
- `type: security`
- `sprint: 08`
- `category: enhancement`
- `state: done`

## Mål

Implementera implementera rbac skeleton som del av model-router.

## Bakgrund

Detta issue stödjer målet att bygga en låg-latency prompt-router som kan välja modell automatiskt, förklarbart och säkert.

## Acceptanskriterier

- Admin/user roles definierade.
- UI/API skyddas.
- Tester finns.

## Tekniska noter

- Bevara fast path latencybudget om ändringen påverkar routing.
- Lägg till strukturerad logging där det är relevant.
- Lägg till eller uppdatera tester.

## Klar när

- Acceptanskriterierna är uppfyllda.
- Tester passerar.
- Dokumentation eller kontrakt är uppdaterade vid behov.

## Implementation (klar 2026-06-12)

- `tenant.Tenant` har explicit `Role` separat från scopes, med `admin` och `user` samt legacy-kompatibilitet för unset role.
- `auth.RequireRole` skyddar admin-only routes och returnerar stabilt `insufficient_role`.
- Dashboard HTML/JSON är admin-only; chat/decision/outcome behåller befintlig scope-baserad åtkomst.
- Tester täcker role checks, legacy behavior, user-role chat och user-role dashboard-deny.
