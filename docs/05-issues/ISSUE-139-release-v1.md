# ISSUE-139 — Första releasen: v1.0.0

- **Epic:** EPIC-14 (drift)
- **Priority:** P1
- **State:** done (2026-10-29)

## Bakgrund

Self-hosters ska kunna pinna en riktig version i stället för `latest` eller
en commit. Version-sidan (ISSUE-138) jämförde bara mot main.

## Ändring

- `buildinfo.Version` stämplas från release-taggen (Dockerfile `ARG VERSION`,
  workflowen skickar `github.ref_name` för taggar, tomt för main).
  `sluss_build_info` får etiketten `version`; MCP `server_info` får
  `build_version`; menyns fot visar `v1.0.0` i stället för commit.
- Image-taggar vid release: `:v1.0.0`, `:1.0` (semver major.minor) och
  `:sha-…`; `:latest` följer fortfarande main.
- **Uppdateringskontrollen följer rätt spår:** en release-build jämförs mot
  senaste GitHub-release (`releases/latest`) och visar "v1.1.0 is out" med
  ändringarna; en main-build jämförs mot main som förut.
- Version-sidan: "v1.0.0 · release · commit …" med länk till release notes.
- `CHANGELOG.md` med v1.0.0.
- Taggen `v1.0.0` och GitHub-releasen skapas på merge-commiten.
