# ISSUE-161 — Leveranskedja: Go 1.26, govulncheck i CI, signerade bilder med SBOM

`epic: EPIC-12` · `priority: P1` · `state: done` · klar 2026-11-07 (#71)

## Varför

NIS2 lägger leverantörskedjekontroll på kunden; de ber om SBOM och signatur.
Dessutom: Go 1.25 underhålls inte längre, och `govulncheck` fann tolv
stdlib-sårbarheter som koden anropar på 1.25.14.

## Levererat

- Go 1.26.10 i go.mod och Dockerfile; CI bygger med toolchainen go.mod anger.
- `govulncheck ./...` i CI på varje ändring (noll fynd på 1.26.10).
- `publish-image` signerar varje bild med cosign (keyless, Sigstore, bunden
  till workflow-identiteten), genererar SPDX-SBOM, attesterar den till bilden
  och bifogar den till releasen för en tagg. Verifierat: `cosign verify` mot
  `:latest` ger identiteten `publish-image.yml@refs/heads/main`.
- Guiden visar `cosign verify` och hur man läser ut SBOM:en.
