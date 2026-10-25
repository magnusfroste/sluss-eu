# ISSUE-133 — Manual för den som inte har gått en kurs

- **Epic:** EPIC-15 (produktyta)
- **Priority:** P1
- **State:** done (2026-10-25)

## Bakgrund

Rollout, SSO och filterguiderna förutsatte att läsaren kan IdP, MDM och
DNS-filter. Ägaren (2026-10-25): "det är svårt för någon som inte har en kurs
i detta".

## Beslut: en källa, tre läsare

Markdown i `manual/` i repot (läsbart för agenter, versionerat med koden),
inbäddat i binären och visat på instansen under `/manual` med `{{BASE}}`
ersatt av instansens adress, och rått på `/manual/<sida>.md`. Fungerar även
air-gappat. www.sluss.eu kan länka till GitHub-versionen.

## Sidor

Start här (fem mål i ordning med tidsåtgång), modeller och första regeln,
stoppa ChatGPT och visa företagets chatt, utvecklarverktyg via Sluss, visa
att det fungerar, ordlista, felsökning. Varje sida: vad det ger, vad du
behöver, tid, numrerade steg, hur du kontrollerar. Inloggningssteget pekar på
SSO-avsnittet i `docs/07-operations/self-host-docker.md`.

## Ändring i koden

- `manual/manual.go`: `embed.FS` + menyordning.
- `internal/server/manualpage.go`: `/manual`, `/manual/{page}`, `.md` rått,
  publikt men noindex, 404 för okända sidor.
- `markdownToHTML`: numrerade listor och länkar (endast `http(s)`, `/`, `#`,
  efter escaping).
- Länkar från Rollout, Quick start, startsidan och nav-posten "Manual".

## Verifierat

Tester: varje sida svarar med ifylld adress och noindex, `.md` rått, länkar
och numrerade steg renderas, okända sidor 404, alla interna länkar pekar på
befintliga sidor och alla filer finns i menyn, `javascript:`-länkar renderas
aldrig.
