# ISSUE-172 — Sidan Audit log med förseglingsknapp, och signaturen kontrollerad

`epic: EPIC-15` · `priority: P2` · `state: done` · klar 2026-11-11

## Varför

Förseglad rotation (ISSUE-171) gick bara att utlösa med en inställning eller
ett admin-anrop. Ägarens villkor står kvar: ingen knapp som raderar
auditloggen. En knapp som förseglar ryms inom det.

## Levererat

- **`/router/audit`**, flik under *Activity*: antal poster, filen i bruk
  (poster, storlek, om och när den förseglas av sig själv), förseglade filer
  med SHA-256, nedladdning av hela loggen, senaste postens nummer och hash.
- **Seal now** (`POST /router/audit/seal`): förseglar filen i bruk och
  fortsätter kedjan i en ny. Förseglingsposten bär den inloggades namn.
  Knappen är avstängd när det inte finns något att försegla (tom logg, eller
  bara fortsättningsposten), och ett andra tryck vägras.
- Inget på sidan raderar; ett test kontrollerar det.
- Evidence-kortet på översikten länkar till sidan.

## Verifierat

Test (`internal/server/auditpage_test.go`) och i webbläsare mot en lokal
instans: inloggning, försegling, listan över förseglade filer, avstängd
knapp efteråt, ingen horisontell rullning i mobilbredd.

## Signaturen på släppt image (kontroll, ingen kodändring)

`cosign verify` och `cosign verify-attestation --type spdxjson` kördes mot
`ghcr.io/magnusfroste/sluss-eu:v1.10.0` med kommandona i driftguiden
(cosign 2.4.1, 2026-11-11):

- Signaturen verifierar; certifikatets identitet är
  `publish-image.yml@refs/tags/v1.10.0`, commit `6f305d4`, digest
  `sha256:2b7a6996…f398ec`.
- SBOM-intyget verifierar: SPDX 2.3, 36 paket, samma digest.
- Med fel identitet (annat repo) misslyckas kontrollen, som den ska.

## Go 1.26.11 (i samma ändring)

CI stoppade ändringen på `govulncheck`: GO-2026-6859 i `net/http`, publicerad
efter v1.10.0, rättad i Go 1.26.11. `go.mod` och `Dockerfile` är uppgraderade
och kontrollen är ren. Releaser till och med v1.10.0 är byggda med 1.26.10.
