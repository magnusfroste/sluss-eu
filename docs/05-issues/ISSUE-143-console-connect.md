# ISSUE-143 — Connect clients i konsolramen

`epic: EPIC-14` · `priority: P3` · `state: done`

## Varför

Fliken "Connect clients" under Rollout pekade på den publika `/connect`, så
admin lämnade konsolen (meny, flikar, designsystem). Receptet sa "paste your
key" utan hjälp, och utan `ROUTER_PUBLIC_URL` visade sidan den gamla
platshållaren `tokenizer.your-company`.

## Vad

- `/router/connect` i konsolramen (Rollout-fliken pekar dit; Quick start
  länkar dit). `/connect` finns kvar publikt för den som inte är inloggad.
- De tre värdena överst med kopieringsknappar: Base URL, Anthropic-bas, modell
  `auto`.
- Fält för API-nyckel som fyller i `<key>` i varje kodsnutt — bara i
  webbläsaren, aldrig skickad, sparad eller loggad. Länk till Access för att
  skapa en avdelningsnyckel (lagrade nycklar är hashar och kan inte visas).
- Klienterna grupperade: Editors, Coding agents, Chat apps, SDKs; varje kort
  med var värdena ska och en kopierbar snutt.
- "Does it work?": ett `curl`-anrop och hänvisning till Request log.
- Utan `ROUTER_PUBLIC_URL` används adressen sidan öppnades med (båda sidorna),
  aldrig det gamla namnet. Aider-receptet använder `--model openai/auto`.

## Klart när

- `connect_test.go`: publik sida med publik URL och med adressen från
  förfrågan, konsolsidan med ram, grupper, snuttar, curl och nyckelfält.
- Kontroll i webbläsare desktop och mobil, nyckeln fylls i snuttarna, inga
  JS-fel.
