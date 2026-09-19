# ISSUE-109 — Repo-split: allowlist-export för publik open-core-kärna

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P1
- **State:** done (2026-07-25)
- **Beroende:** ADR-0014 (open core + två-repo-modell), DECISION_LOG 2026-07-19.

## Bakgrund

Repot var kortvarigt publikt med hela GTM-korpusen (deck, strategi,
advisor-identitet) — disciplin räckte inte. Beslutet (ADR-0014): privat repo =
enda sanningskällan; publikt repo = **genererad, enkelriktad allowlist-export**.
Denna issue implementerar mekaniken.

## Levererat

- **`scripts/publish-public.sh`** — allowlist-export (fail-closed: listar vad
  som GÅR ut, inte vad som exkluderas). Två lägen: `--check` (torr körning +
  guard, CI-säker) och `--out DIR` (bygger export-trädet från git-trackade
  filer, så otrackad skräp aldrig följer med).
- **Tre guard-lager** (defense in depth):
  1. Allowlist — primärt skydd; ej listad → ej exporterad.
  2. Path-guard — failar om en förbjuden sökväg hamnar i export-setet (fångar
     en framtida redigering av allowlisten själv).
  3. Content-guard — skannar exporterade filer efter *riktiga* nyckel-format
     (PEM, `sk-or-v1-…`, `sk_sluss_admin_…`, Z.ai-format) och advisor-/GTM-
     advisor-identifierare. Dev-defaults (`local_router_key`,
     `demo`, platshållare) är medvetet publika och triggar inte.
- **CI-steg** (`ci.yml`): `publish-public.sh --check` körs på varje PR → en
  hemlighet eller identifierare som råkar läggas i en *allowlistad* fil failar
  bygget före merge.
- **`PUBLISHING.md`** — intern runbook (stannar privat): allowlist-rationale,
  vad som går ut/stannar, steg för att skapa/uppdatera publika repot med ren
  historik, AGPL-noten, one-way-regeln.
- **ADR-0014** — cementerar open core (AGPL-kärna + `ee/`-gräns),
  två-repo-modellen och provider-opt-in.

## Verifiering (guard-matris, körd)

- Ren repo → OK, 361 filer, exit 0.
- Förbjuden sökväg tvingad in i allowlisten → GUARD FAIL, exit 1.
- Advisor-fras injicerad i allowlistad fil → GUARD FAIL, exit 1.
- Fejk `sk-or-v1-…`-nyckel injicerad → GUARD FAIL, exit 1.
- Export-träd (`--out`) innehåller noll interna filer (DECISION_LOG,
  00-product, .ai, CLAUDE/AGENTS, 05-issues alla frånvarande); produktkod +
  publika docs närvarande.
- **Självrefererande läcka (fixad):** guarden flaggade sig själv när skriptet
  blev git-trackat — identity-mönstren låg som literaler i
  ett skript som exporteras. Att publicera guarden hade läckt exakt det den
  skyddar. Flyttat till privat, icke-allowlistad `.publish-guard/identities.txt`
  (saknas filen = identitetsskanning av, path/secret-guards kvar). Tom
  identity-regex passeras aldrig till `grep -e` (skulle matcha varje rad).
- **Buggfix under bygget:** content-guarden var först tyst trasig — `grep`
  tolkade ledande `-----BEGIN` som en flagga (exit 2, dolt av `2>/dev/null`),
  så inget innehåll skannades. Fixat med `grep -e` (explicit mönster).
  Institutionaliserad läxa: en guard som aldrig kan *fela* måste negativt-testas.

## Utanför omfattning (ägar-/framtidssteg)

- Skapa det faktiska publika repot + fresh-history-push (körs av ägaren enligt
  PUBLISHING.md; kräver GitHub-åtkomst).
- LICENSE (AGPL-3.0-only, via GitHubs licens-picker för kanonisk text),
  SECURITY.md, CONTRIBUTING.md läggs i det publika trädet.
- `ee/`-katalogen (premium) — byggs när första premium-funktionen (SSO,
  schemalagda rapporter) implementeras; allowlisten exkluderar den redan
  (ej listad).
- CI-guard-workflow i det *publika* repot (redundant kopia av path/secret-scan)
  när repot finns.
