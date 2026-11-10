# ISSUE-171 — Förseglad rotation av auditloggen

`epic: EPIC-15` · `priority: P2` · `state: done` · klar 2026-11-10

## Varför

Auditkedjan växer för alltid i en fil. Den får aldrig raderas, men en fil
som bara växer blir till slut ohanterlig att säkerhetskopiera och exportera.
Ägarens villkor sedan tidigare: ingen knapp som raderar auditloggen; filen
flyttas och arkiveras.

## Levererat

- **Försegling:** filen som stängs får en sista post `audit.sealed`, flyttas
  till `audit-archive/` i datakatalogen (`audit-chain-<första>-<sista>.jsonl`)
  och görs skrivskyddad. Den raderas aldrig.
- **Fortsättning:** nya filens första post `audit.continued` fortsätter
  numreringen, länkar till förseglingspostens hash och bär SHA-256 för hela
  den förseglade filen. Filerna i följd är alltså en obruten kedja, och en
  arkiverad fil är låst av en hash som ligger inne i kedjan.
- **Inget ändras för granskaren:** exporten ger hela kedjan från post 1;
  ankare fungerar som förut.
- **`audit-verify`** tar flera filer i ordning och kontrollerar varje
  förseglad fil mot hashen i kedjan; en ensam fortsättningsfil verifieras
  internt med en upplysning om vilken fil den fortsätter från.
- **Utlösning:** `ROUTER_AUDIT_ROTATE_MB` (av som standard) eller
  `POST /router/audit/rotate`. `GET /router/audit/segments` listar filerna.
- **Robusthet:** två processer på samma fil (överlappande utrullning) forkar
  inte kedjan när den ena roterar; en rotation som avbryts efter flytten
  fortsätter kedjan vid nästa start i stället för att börja om på 1.

## Verifierat

Enhetstester (`internal/audit/rotate_test.go`, även med `-race`) och en
lokal körning: fem rotationer via storlek och en för hand, export 24 poster
verifierad, filerna verifierade en och en och tillsammans, en ändrad
arkivfil avslöjad, ankare kontrollerat mot exporten.

## Avgränsning

- Ingen knapp i konsolen; förseglingen görs av inställningen eller ett
  admin-anrop.
- Arkivet ligger på samma disk. Att flytta förseglade filer till annan
  lagring är operatörens sak; hashen i kedjan gör att de kan kontrolleras
  var de än ligger.
