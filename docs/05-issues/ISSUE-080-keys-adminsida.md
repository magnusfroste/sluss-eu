# ISSUE-080: Keys-adminsida (mint/lista/revoke i UI)

## Labels
- `epic: EPIC-14`
- `priority: P1`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Mål

En "Keys"-sida i admin-navigationen (samma skal som Models/Providers) där en
administratör skapar en nyckel för en avdelning, ser listan och återkallar —
utan curl. Detta är vad som visas i pitchen när "olika avdelningar, olika
nycklar" demonstreras.

## Design

- Ny nav-post `🔑 Keys` i `adminNavItems`; sida byggd med befintligt
  admin-skal + tabellmönster (som Providers-sidan).
- Tabell: key_id, tenant/projekt, roll, scopes, skapad, senast använd, status
  (aktiv/återkallad) + revoke-knapp med bekräftelse.
- "Skapa nyckel"-formulär: tenant-id, projekt, roll, scopes →
  klartextnyckeln visas **en gång** i en kopierbar ruta med tydlig varning
  ("visas aldrig igen").
- Anropar admin-API:t från ISSUE-079; samma auth-gate som övriga adminsidor
  (Bearer admin-scope eller dashboard-lösenord).

## Acceptanskriterier

- Mint → nyckeln syns en gång, dyker upp i listan; revoke → status ändras och
  nyckeln slutar fungera.
- Sidan följer adminskalets utseende och fyller browserbredden (som övriga).
- Handler-tester för sidans endpoints (RBAC: icke-admin nekas).
- Tom-tillstånd: "Inga nycklar ännu — skapa den första" + bootstrap-nyckelns
  existens förklaras.
