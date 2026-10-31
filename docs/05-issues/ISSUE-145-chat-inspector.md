# ISSUE-145 — Inspector i chatten

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Routing-etiketter, PII-notis och policyblock låg i chattflödet och tävlade med
svaret. Inspirerat av Claude Code: flödet visar svaret, en panel till höger
visar vad som bestämdes och resultatet.

## Vad

- **I flödet:** en kompakt rad per svar — `● glm-local · stayed local ·
  personal data` (grön/blå/röd prick för lokal/moln/blockerad). Klick väljer
  svaret i panelen. Policyblocket är kvar som kort i flödet.
- **Inspector → This answer:** utfall (stannade i huset / moln / blockerad),
  uppgift, risk, data (och om den kom från tidigare i chatten), regler som
  matchade och policyversion, modell, leverantör, lokal/moln med taggar,
  reserv, tid till första ord och total, tokens och kostnad, webbsök, bilagor,
  request-id. Bläddra Earlier/Later mellan svaren.
- **This chat:** stapel lokal/moln/blockerad, antal svar, modeller, kostnad
  och tokens.
- **Settings:** modellväljaren (flyttad från rubriken), webbsök, hur routning
  går till.
- **Anställda** ser den enkla versionen: utan regel-ID, leverantör, kostnad och
  request-id.
- ≥1280 px en kolumn som minns om den är öppen; smalare en låda från höger
  (Escape/klick utanför stänger).
- Blockerade svar sparas nu i historiken (med orsak) men skickas aldrig vidare
  till en modell.

## Backend

- `RouteDecision.MatchedRules` (policyregler som matchade).
- Nya svarshuvuden: `X-Router-Risk`, `X-Router-Selected-Provider`,
  `X-Router-Policy-Rules` — ASCII-tokens, aldrig innehåll.
- `GET /chat/request?id=` ger tokens och kostnad efter ett strömmat svar,
  bara för den egna förfrågan (actor = inloggad), aldrig innehåll eller person.
- In-memory request-loggen bär nu `TenantID`/`Actor` som den durable gör.

## Klart när

- Tester: huvudena är rena ASCII-tokens (även med CRLF och icke-ASCII i
  regelnamn), `/chat/request` svarar bara på egen förfrågan.
- Webbläsare 1600/1100/390 px: svar och block visas rätt i panelen, tokens
  fylls i efteråt, val via raden i flödet och Earlier/Later.
