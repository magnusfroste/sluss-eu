# ISSUE-128a — Rollout-kit: MDM, DNS-filter, blocksida, coverage

- **Epic:** EPIC-12 (compliance-yta)
- **Priority:** P1
- **State:** done (2026-10-14)

## Bakgrund

Sluss kräver att klienter pekar på vår endpoint. Insikten (2026-10-13): en
MDM (Intune, Jamf, Kandji, Fleet) kan sätta `OPENAI_BASE_URL` och
`ANTHROPIC_BASE_URL` på varje hanterad enhet, så att "utvecklaren måste komma
ihåg" försvinner. Chattfönster (chatgpt.com) kan inte styras om (TLS), bara
blockeras och ersättas: kundens DNS-filter/SWG + blocksida + Sluss-chatt.
Bevis per enhet utan egen agent: MDM:en kör redan skript på schema.
Analysen av MDM/EDR-krockar, Windows/Mac/Linux och MBL/Betriebsrat ligger i
sessionen 2026-10-13 och styrde avgränsningen: ingen binär, ingen drivrutin,
ingen inspektion.

## Ändring

- **`/router/rollout`** (konsolen, nav "Rollout" under Control): tre steg
  med nedladdningar, genererade från `ROUTER_PUBLIC_URL`:
  1. *Point the tools here:* `sluss-env.sh` (macOS: `/etc/zshenv` +
     LaunchAgent som kör `launchctl setenv`; Linux: `/etc/profile.d`) och
     `sluss-env.ps1` (Machine-env). Nyckeln sätts aldrig i skriptet.
  2. *Block + offer:* `genai-domains.csv/json` (22 domäner, deny; Sluss-
     värden allow; M365 Copilot medvetet utelämnad), `sluss-webclip.
     mobileconfig` (stabila UUID:er så omnedladdning är samma profil),
     `sluss-browser-policy.json` (ManagedBookmarks + `DnsOverHttpsMode=off`).
  3. *Prove it, per device:* `coverage.sh`/`coverage.ps1`. Öppen väg =
     en leverantörs-API svarar 401 (riktig leverantör; sinkhole/blocksida
     gör aldrig det). Rapporterar hostname, OS, `direct_ai`, `sluss` —
     aldrig användare, aldrig innehåll. PS1 avslutar med 1 vid öppen väg
     så Intune kan markera enheten som non-compliant.
- **`POST /router/coverage`**: bakom `auth.Middleware` (avdelningsnyckel)
  → tabell `coverage_devices`, en rad per enhet, senaste vinner, attribuerad
  till nyckelns tenant.
- **`GET /blocked`**: publik, noindex, `?from=<host>` visas escapat, länk
  till `<PublicURL>/chat`.
- **Dashboard:** kort "Managed devices: M / N still reach AI providers
  directly" när minst en enhet rapporterat; länkar till Rollout.
- README: nytt avsnitt "Fits your IT — no new island" (IdP, MDM, SWG,
  audit). Self-host-guiden pekar på Rollout.

## Verifierat

Enhetstester (`rollout_test.go`): 401 utan nyckel, 400 på fel payload,
upsert per enhet och summering, sidan listar enheter och filer, alla filer
bär instansadressen och ingen nyckel, denylistan nekar leverantörer och
tillåter instansen, coverage-skripten letar 401-signaturen, web clip
byte-identisk mellan nedladdningar, blocksidan escapar `from`. Rökprov mot
riktig binär: login → rollout → nedladdning → coverage 204 → enhetsrad +
dashboardkort → blocksida 200, inga fel i loggen.

## Medvetet utanför

Egen enhetsagent (128b), TLS-interception (aldrig), coverage från MDM:ens
egen app-inventering (mobil), SCEP/mTLS-identitet.
