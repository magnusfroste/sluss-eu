# ISSUE-147 — Inspector i flikar: Route, Context, Output, Setup

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Inspector (ISSUE-145) blandade två frågor i en lång lista — "vad hände med det
här svaret?" och "vad gäller i chatten?" — och visade inte vad som faktiskt
skickades. Inspirerat av Claude Codes kategorier.

## Vad

- **Sammanfattning överst** (alltid synlig): stapel lokal/moln/blockerad,
  antal svar, modeller, kostnad.
- **Route:** utfall, uppgift med klassificerarens säkerhet ("34 % sure —
  unclear prompts are treated with more care"), risk, data, regler, version,
  utfall, modell, leverantör, lokal/moln, reserv.
- **Context (ny):** hur många meddelanden som skickades och hur många som hölls
  tillbaka, promptens storlek, om datan kom från tidigare i chatten, vad
  kontextpipelinen kortade, bilagor och webbkällor (med länkar).
- **Output:** tid till första ord och total, svarets längd, tokens, kostnad
  (`$0 · own hardware` lokalt), request-id och auditlänk.
- **Setup:** modellväljare, webbsök, routningsläge, vem du är, instansens
  version.
- Fliken minns sitt val; klick på svarsraden öppnar Route; piltangenter byter
  flik. Anställda ser bara Route och Output.
- **Blockerade turer hålls tillbaka:** frågan som blockerades och blocket
  skickas aldrig igen, så en hemlighet i ett blockerat meddelande inte följer
  med eller fortsätter klassificera resten av chatten. Context visar antalet.

## Backend

- `X-Router-Task-Confidence` och `X-Router-Prompt-Tokens` (bara siffror).
- Chattsidans flaggor `WHO`, `DEPT`, `VERSION` JSON-kodas så ett namn från
  identitetsleverantören inte kan bryta ut ur skriptet.

## Klart när

- Tester: nya huvuden, flaggorna escapas (`</script>` i namnet).
- Webbläsare: varje flik, tre turer där den blockerade hölls tillbaka (1, 3, 3
  meddelanden skickade).
