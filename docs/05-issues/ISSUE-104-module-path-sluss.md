# ISSUE-104 — Modulsökväg efter repo-rename: `magnusfroste/tokenizer` → `magnusfroste/sluss`

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P1
- **State:** done (2026-07-19)
- **Beroende:** ISSUE-103 (rebrand av användarriktad yta), ägarbeslutet att döpa om GitHub-repot (utfört 2026-07-19).

## Bakgrund

ISSUE-103 bytte all användarriktad yta till Sluss men lämnade Go-modulsökvägen
och repo-namnet som separata ägarbeslut (DECISION_LOG 2026-07-18). Repot är nu
omdöpt till `github.com/magnusfroste/sluss`; GitHub redirectar gamla URL:er,
men modulsökvägen ska följa repot så att `go get`/pkg-referenser pekar rätt.

## Omfattning

- `go.mod`: `module github.com/magnusfroste/sluss`
- Alla interna imports (187 Go-filer): `github.com/magnusfroste/tokenizer/...` → `github.com/magnusfroste/sluss/...`
- `deploy/docker-compose.yml`: ghcr-imagereferens → `ghcr.io/magnusfroste/sluss` (publish-workflowen använder `${{ github.repository }}` och följer renamet automatiskt)
- `Dockerfile`: build/run-exempel `tokenizer-router` → `sluss-router`

## Utanför omfattning

- Historiska omnämnanden i DECISION_LOG och äldre issues (historik ändras inte)
- `tokenizer.froste.eu` (live-URL:en är faktisk tills DNS-flytten till sluss.eu är klar — eget ägarbeslut)
- Postgres-defaultsträngen i Makefile (legacy, appen kör SQLite)

## Acceptans

- [x] `go build ./...` och `go vet ./...` rena
- [x] Full testsvit grön (CI; lokalt fallerar endast `TestFeatureExtractionLatency` p.g.a. långsam sandbox — verifierat att den fallerar identiskt på orörd main)
- [x] Inga kvarvarande `github.com/magnusfroste/tokenizer`-referenser i Go-kod/go.mod
