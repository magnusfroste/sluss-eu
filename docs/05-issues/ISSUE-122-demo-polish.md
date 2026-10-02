# ISSUE-122 — Demo-polish: rapporter som sidor, chatten på mobil, fallback synlig, konversationskontext

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P1 (nästa demo = demo av demo.sluss.eu)
- **State:** done (2026-10-02)

Resultat av UX-svepet 2026-10-02 (alla 14 sidor, desktop + 390 px).

## Ändring

1. **Rapporterna är riktiga sidor.** `/router/incident-report`,
   `/router/gap-report` och `/router/compliance/report` renderas i konsolens
   tema när en webbläsare frågar (Accept: text/html): nyckeltal som kort
   (requests, blockerat, känsligt lokalt/moln …), rapporttexten med rubriker,
   listor, tabeller och kodblock, knappar **Download as Markdown** och
   **JSON**. curl, agenter (MCP) och `?format=md` får samma Markdown som
   förut — oförändrad; `?format=json` orört. Konverteraren
   (`markdownToHTML`) täcker exakt den dialekt rapportskrivarna använder och
   HTML-escapar allt innehåll först. Kontrollrapportens nedladdningsnamn:
   `control-report.md` (var `kontrollrapport.md`).
2. **Chatten på mobil.** Sessionspanelen döljs under 760 px, rubriken bryter
   rätt, modellväljaren tar hela bredden, bubblorna får hela bredden.
   Demolänkar öppnas ofta på en telefon först.
3. **Dashboarden på mobil.** Tabellerna scrollar inuti sin ruta i stället
   för att dra ut hela sidan; besparingssiffran skalas ner. Alla konsolsidor
   är nu fria från horisontellt överflöd i 390 px.
4. **Fallback syns.** Svaret bär `X-Router-Fallback-Index` och nytt
   `X-Router-Primary-Model` på både streaming- och icke-streamingvägen; chatten
   visar brickan *fallback · from <primär>* med förklaring (kedjan byggs före
   första anropet och vidgar aldrig egress). Tidigare syntes bara vinnaren.
5. **Konversationskontext.** Hela konversationen klassas, så "write a commit
   message" efter ett personnummer stannar lokalt — rätt, men såg ut som ett
   fel. Ny header `X-Router-Sensitivity-Source: message|conversation`
   (klassning av senaste användarmeddelandet ensamt jämförs med hela
   konversationen; klasser, aldrig innehåll). Chatten säger då
   "personal data detected (personnummer) **earlier in this conversation** —
   the whole conversation stays in the house." Sätts även vid block.

## Tester

- `TestReportContentNegotiation` (browser → HTML, curl → Markdown,
  `?format=md` tvingar, `?format=json` orört), `TestMarkdownToHTMLCoversReportDialect`
  (inkl. escaping av `<script>`).
- `TestSensitivitySourceHeader` (ensamt meddelande: ingen header; uppföljning
  efter PII: `conversation`; PII i senaste: `message`).
- Streaming-vägen verifierad med curl; skärmdumpar av rapportsidorna,
  chatten (desktop + mobil) och dashboarden i 390 px; överflödsmätning på
  åtta sidor.
