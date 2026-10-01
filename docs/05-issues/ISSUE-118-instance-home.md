# ISSUE-118 — Instansens startsida: produkten på en rad + Sign in (marknad till www.sluss.eu)

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P2
- **State:** done (2026-10-01)

## Bakgrund

Ägarbeslut 2026-10-01: marknadsföringen flyttade till en separat produktsajt,
`www.sluss.eu`. Den som laddar ner och kör Sluss vill få igång den och logga
in — inte läsa en säljsida. Separation of concerns: produktsajten säljer,
instansen är ett operatörsverktyg.

## Ändring

- **`/` är en instans-startsida:** vad det här är (en rad), **Sign in** som
  primär knapp, *Connect a client*, och fyra *Getting started*-steg
  (logga in, provider + modell med `local`-tagg, välj policy + dry-run, peka
  en klient hit). Sidfot: *About Sluss* (produktsajten), *Docs & source*,
  *Security*, AGPL-3.0, "not a legal compliance certification".
- **Bort:** riskavsnitt, positionering, "How it works", FAQ, JSON-LD
  (SoftwareApplication/FAQPage), OpenGraph/Twitter-kort, canonical,
  `/sitemap.xml` och `/llms.txt`. Copyn finns kvar i git-historiken.
- **Indexeras inte:** `robots.txt` = `Disallow: /`, `<meta name="robots"
  content="noindex, nofollow">` och `X-Robots-Tag`. En självhostad gateway ska
  inte hittas av sökmotorer, och demo-instansen ska inte konkurrera med
  produktsajten.
- README: startsidan beskriven, länk till produktsajten.

## Driftnot (sluss.eu)

`demo.sluss.eu` och `sluss.eu` pekar båda på demo-instansen; `www.sluss.eu`
är produktsajten. Rekommendation: behåll `demo.sluss.eu` som instansens namn
(ärligt — "app." signalerar SaaS, vilket motsäger self-hosted), sätt
`ROUTER_PUBLIC_URL=https://demo.sluss.eu`, och låt apex `sluss.eu` 301:a till
`www.sluss.eu` först när alla klienter (MCP-agenten, demo-nycklar) bytt URL.

## Tester

`TestLandingIsInstanceHome` (Sign in primär, getting started, noindex, ingen
SEO-markup, ingen länk till /chat), `TestRobotsKeepsInstanceOutOfIndex`,
`TestRootServesLandingPage` (sitemap/llms.txt borta). Skärmdumpar desktop +
mobil.
