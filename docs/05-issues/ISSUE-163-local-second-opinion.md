# ISSUE-163 — Lokal andra åsikt som bara får skärpa

`epic: EPIC-14` · `priority: P2` · `state: done` · klar 2026-11-07

## Idé

Reglerna är golvet och förblir den primära förklaringen. En liten
klassificeringsmodell (NER/PII, inte en generativ LLM) på egen hårdvara får
säga "det här ser ut som hälsodata" eller "det här är personuppgifter", varpå
klassen **höjs**. Den får aldrig sänka en klass. Dess utslag loggas bredvid
regelns och syns i inspektorn som "second opinion: health (local model)".

## Varför

Korpusen i ISSUE-162 visar var reglerna slutar: betydelse utan identifierare
(ett namn, ett förvärv i klartext, en sjukfrånvaro utan hälsoord). Zscaler,
Netskope och Purview använder ML för exakt det. Hybrid behåller USP:n: samma
fail-closed, samma förklarbarhet, inget data lämnar maskinen.

## Krav om det byggs

- Får bara höja. Regeln är alltid primär i `decision_reasons`.
- Lokal: modellen kör på operatörens hårdvara; av om den inte finns.
- Latensbudget: p95 < 100 ms totalt → en liten modell, inte en chatt-LLM.
- Mätt i korpusen: siffrorna på What Sluss catches får två kolumner.
- Av som standard; en inställning, inte tio.

## Byggs när

En kund ber om täckning på betydelse och kan köra en liten modell lokalt.

## Levererat (2026-11-07)

- `internal/secondopinion`: frågar en rostermodell taggad *local* (annars vägrat)
  vilka dataklasser texten håller, med fast vokabulär och JSON-svar; parsningen
  räknar bara vokabulärens ord. `job.RaiseSensitivity` höjer klass och
  riskgolv, sänker aldrig. Timeout 500 ms (`ROUTER_SECOND_OPINION_TIMEOUT`);
  därefter gäller reglernas beslut ensamt.
- Körs efter reglerna, före policy och beslutscache, i chat-vägen, `/router/decision`,
  policyns dry-run och MCP `route_explain`. Skäl i `decision_reasons`, headers
  `X-Router-Second-Opinion`/`-Model`, rad i inspektorn. Mätvärde
  `router_second_opinion_total{outcome}`.
- Av som standard; en inställning: `ROUTER_SECOND_OPINION_MODEL`.
- Kvar (backlogg): korpusen med två kolumner (med/utan andra åsikt) kräver en
  körande lokal modell i CI; mäts tills vidare för hand.
