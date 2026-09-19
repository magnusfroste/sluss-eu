# ISSUE-112 — Open source-cutover: en publik utvecklarrepo

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P0
- **State:** done (2026-09-19)
- **Ersätter:** ISSUE-109 (allowlist-export), del av ADR-0014 (två repos, `ee/`).

## Bakgrund

Kundanskaffningen behöver att tekniker kan ladda ner, läsa och testa själva —
"inget att dölja". Ägarbeslut (DECISION_LOG 2026-09-19): en publik repo med
kod, arkitektur, ADR:er, backlogg och research, under AGPL-3.0. Två-repo-
modellen utgår.

## Inventering före öppning

- **Hemligheter i hela git-historiken:** 0 träffar (nycklar, lösenord,
  providernycklar). Rent.
- **Tredjeparts-identiteter/relationer i trädet** (måste bort): advisor-
  identitet (DECISION_LOG, doc 11, deck, ISSUE-109, `.publish-guard`), ett
  prospekt/CISO-möte + LinkedIn-tråd (`.scratch/pitch/`), en inbound
  partnerförfrågan från en namngiven leverantör (doc 14), eget CRM-verktyg
  vid namn (7 filer), investerardecket (fundraising-artefakt med advisor-referenser).
- Samma namn finns i ~5 commits i historiken → **repot öppnas med ny historik**
  (gamla repot fryses privat).

## Genomfört

- **Avidentifierat:** advisor → "erfaren CISO från reglerad finanssektor";
  leverantören i doc 14 → "en välfinansierad enterprise-agentleverantör"
  (bolagsfakta och källänkar som identifierar dem strukna); CRM-verktyget →
  "externt CRM" i docs, index, `docs/mcp.md` och två Go-kommentarer.
- **Borttaget:** `.scratch/pitch/` (till ägarens privata vault),
  `docs/00-product/12-investor-deck.md` (fundraising-artefakt, till vaulten),
  `.publish-guard/`, `PUBLISHING.md`, `scripts/publish-public.sh`
  (allowlist-maskineriet är meningslöst när allt är publikt).
- **Tillagt:** `LICENSE` (AGPL-3.0-only, kanonisk text), `SECURITY.md`,
  `CONTRIBUTING.md`, `scripts/check-secrets.sh` + CI-steg (secret-scan av
  varje PR — ersätter allowlist-guarden), README-licenssektion,
  `docs/07-operations/open-source-release.md` (cutover-runbook).
- **Beslut:** DECISION_LOG-rad 2026-09-19; ADR-0014 status-amendment.

## Medvetet kvar (publikt)

Demo-manuset med live-facit (sluss.eu är publikt; modellnamn är ofarliga),
positionering/marknadsresearch (doc 08–11, 13, 14 — konkurrentnamn är
produktjämförelser, inte relationer), backlogg/issues/ADR:er, `.ai/`-state,
`CLAUDE.md`/`AGENTS.md` (agent-konventioner), grundarens namn i
produktbriefen (repot är hans).

## Acceptans

- [x] `grep` över hela trädet för person-/bolags-/relationstermer: 0 träffar
- [x] `scripts/check-secrets.sh`: rent
- [x] `go build ./...` + testsvit gröna
- [x] Runbook för ny-historik-push levererad (ägarsteg)
