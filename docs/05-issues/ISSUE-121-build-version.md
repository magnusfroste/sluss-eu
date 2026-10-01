# ISSUE-121 — Synlig buildversion: vilken commit kör instansen?

- **Epic:** EPIC-14 (drift)
- **Priority:** P2
- **State:** done (2026-10-01)

## Bakgrund

Fyra av sju deployer i EasyPanel landade inte — containern behölls med den
gamla imagen utan felmeddelande. Det enda sättet att upptäcka det var
processens starttid i `/metrics` jämfört med merge-tiden. Instansen ska själv
kunna säga vilken build den kör.

## Ändring

- Nytt paket `internal/buildinfo`: `Commit` och `Built` stämplas vid länkning
  (`-ldflags -X`). En vanlig `go build` i en git-checkout faller tillbaka på
  Gos VCS-stämpel (`vcs.revision`, `-dirty` vid ändringar); annars `dev`.
- **Dockerfile:** `ARG GIT_SHA` och `ARG BUILD_TIME` går in via ldflags.
  **publish-image-workflowen** skickar `github.sha` och commit-tiden.
- **Synligt på fyra ställen:**
  - `/metrics`: `sluss_build_info{commit="…",built="…"} 1`
  - konsolens sidfot: `admin · 3d79212`, länkad till commiten på GitHub,
    byggtid i tooltip (ersätter det hårdkodade "v1.0")
  - MCP `server_info`: `build_commit`, `build_time`
  - startloggen: `router starting … commit=… built=…`

Commiten är inte hemlig (öppen källkod); `/metrics` exponerade redan
processens starttid.

## Verifiering efter deploy

```bash
curl -s https://demo.sluss.eu/metrics | grep sluss_build_info
```
ska visa samma commit som senaste mergen på main.

## Tester

`TestShortTrimsAndKeepsDirty`; stämplad binär provkörd lokalt — rätt commit i
`/metrics`, sidfoten, MCP `server_info` och startloggen.
