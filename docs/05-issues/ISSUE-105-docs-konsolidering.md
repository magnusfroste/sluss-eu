# ISSUE-105 — Repo-struktur: samla dokumentationskorpusen under `docs/`

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-07-19)
- **Beroende:** ISSUE-104 (repo-/modulrename klart, inga öppna PR:ar som får konflikter).

## Bakgrund

Sedan basen byggdes har repo-roten haft nio numrerade dokumentkataloger
(`00-product/` … `08-templates/`) bredvid koden, plus en separat `docs/` med
guider, agent-konventioner och notes. Roten såg ut som ett dokumentationsprojekt
med koden begravd — fel första intryck för design partners och teknisk due
diligence. Ägarbeslut 2026-07-19: konsolidera under `docs/`.

## Omfattning

- `git mv` av alla nio numrerade katalogerna till `docs/00-product/` …
  `docs/08-templates/` — som ett block, med numrering och interna relativa
  korslänkar intakta (historik bevaras via rename-detection).
- Referenssvep i filer utanför korpusen: `README.md`, `CLAUDE.md`, `AGENTS.md`,
  `MANIFEST.md`, `DECISION_LOG.md` (sökvägsreferenser, inte beslutsinnehåll),
  `cmd/router/main.go`, `internal/policy/types.go`, `internal/server/landing.go`
  (kommentarer), `examples/demo-ciso/README.md`, `.ai/tasks.json`.
- Trädblocken i `README.md`/`CLAUDE.md` omskrivna till `docs/`-wrapper-form.

## Utanför omfattning / medvetet orört

- Filer *inuti* `docs/` (agents, notes, compliance-report, sprint-01-demo):
  deras `00-product/…`-referenser pekar efter flytten på syskonkataloger och
  behöver inte prefix.
- `.ai/ledger.jsonl` — append-only-journal, historik skrivs aldrig om.
- Interna omnämnanden i gamla issues/ADR:er (relativa länkar överlever
  blockflytten).
- Ingen intern omdöpning/sammanslagning av de numrerade katalogerna —
  numreringen bär läsordningen och är refererad i hela issue-/ADR-korpusen.
- `bin/` visade sig redan vara gitignorerad (lokal build-output, aldrig
  trackad) — ingen åtgärd.

## Acceptans

- [x] Repo-roten innehåller endast kod-/infra-kataloger + toppnivå-md
- [x] `go build ./...` och full testsvit gröna
- [x] Inga referenser till gamla rot-sökvägar utanför `.ai/ledger.jsonl`
