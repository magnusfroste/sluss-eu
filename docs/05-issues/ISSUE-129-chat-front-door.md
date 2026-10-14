# ISSUE-129 — Sluss Chat som sanktionerad entrédörr

- **Epic:** EPIC-15 (demo/produktyta)
- **Priority:** P1
- **State:** done (2026-10-14)

## Bakgrund

Block + erbjud (ISSUE-128a) förutsätter att det som erbjuds duger. Folk
använder ChatGPT för sök, filer och historik. Historik per användare kom
med SSO (ISSUE-130). Det här ger de två andra, på Sluss villkor: inget
lämnar gatewayen utan att vara klassat först.

## Ändring

- **Anställd-vy.** `DemoChatPageHandler` renderar per roll: `user` får
  den rena sidan (ingen admin-nav, ingen modellväljare) med tagline
  "company AI", namn · avdelning · Sign out i headern, egen intro och
  hint. Demo-gäst och admin oförändrade.
- **Bilagor.** 📎 i composern. Text/kod (lista på ändelser, ≤300 KB) läses i
  webbläsaren och skickas som array-content `{"type":"text"}`-delar →
  klassas som dokumenttext (ISSUE-102). Bilder (≤2 MB) som `image_url` →
  `requires_vision` → policyn avgör. PDF/Office: "klistra in texten" tills
  vidare. Delarna sparas i sessionen så följdfrågor om filen fungerar.
- **Webbsök.** 🌐-toggle per meddelande (`"search": true`). Middleware
  (`websearch.go`) på `/chat/send`:
  1. klassar senaste användarmeddelandet deterministiskt (samma
     extractor som routern); all känslighet ≠ `none` → `X-Router-Search:
     skipped-sensitive;<klass>`, inget lämnar gatewayen;
  2. annars GET `ROUTER_SEARCH_URL?q=…&format=json` (SearXNG-kompatibel,
     8 s timeout), max 5 resultat;
  3. resultaten injiceras som system-meddelande med numrerade källor, på
     rå-JSON-nivå så array-content och modell-pin passerar oförändrat;
  4. `X-Router-Search: done;N` + `X-Router-Sources` (base64url JSON) så
     svaret visar källor; `failed`/`empty`/`off` hanteras i UI.
  Audit `chat.search`: actor, utfall, antal — aldrig frågan.
- Routingbeslutet är orört: det berikade requestet går genom klassare,
  policy och motor som vanligt.
- Env: `ROUTER_SEARCH_URL` i `.env.example`, `deploy/example.env`, compose.
  Self-host-guiden: avsnitt om chatten som entrédörr.

## Verifierat

`websearch_test.go` med fejk-SearXNG: injektion + källor + strippad
`search`-flagga + audit utan query; PII-prompt når aldrig motorn och
forwardas oförändrad; off/ej begärd/nere/tomt; array-content byte-för-byte.
`TestChatPageVariantsByRole`: anställd utan nav/picker men med identitet,
admin med nav, demo-gäst oförändrad.

## Medvetet utanför

PDF/Office-extraktion (kräver bibliotek eller tjänst), serverlagrade filer,
sök-API:er i molnet (Brave/Tavily går att peka på om de talar SearXNG-
format, men rekommendationen är self-hosted), citat-verifiering.
