# ISSUE-113 — Fallback-kedjan exekveras även för icke-streamande anrop + döda providers räknas inte som spend

- **Epic:** EPIC-06 (provider execution & fallback)
- **Priority:** P0 (hittad vid demo-förberedelse 2026-09-29)
- **State:** done (2026-09-29)

## Bakgrund (live-fynd)

Hälsokontroll inför demo: primär cloud-provider gav 429 (kvot) och lokala
DGX-endpointen var onåbar (DNS). Två saker gick fel i *produkten*:

1. **Ingen fallback för icke-streamande anrop.** Engine bygger kedjan före
   första provideranropet (CLAUDE.md-invariant) — men bara streaming-vägen
   (`streamWithFallback`) gick igenom den. Den routade non-streaming-vägen
   gjorde ett enda `adapter.Complete()` och skrev felet rakt till klienten.
   `recent_errors` visade bara `attempt_index: 0`. En 429 på scen dödar en
   demo-beat trots att gpt-4o-mini stod redo i kedjan.
2. **Misslyckade requests räknades som spend.** Decision-raden i SQLite skrivs
   med pre-call-*estimatet*; bara lyckade attempts nådde `fillAttempt`, så en
   död provider lämnade estimatet kvar → dashboarden visade **−8456 %** savings
   (actual >> baseline eftersom baseline räknas på faktiska tokens = 0).

## Ändring

- `buildCompleteCandidates(dec, adapters)` — primär + engine-fallbacks (redan
  policy-filtrerade, så kedjan vidgar aldrig egress), utan adapter → skippas,
  dubbletter kollapsar.
- Routade non-streaming-vägen går igenom kedjan: klona request per kandidat,
  per-modell reasoning-direktiv, `recordAttempt(i)` per försök (health + audit
  per attempt), första lyckade svarar. Vid fallback sätts
  `X-Router-Selected-Model` till modellen som *svarade* och
  `X-Router-Fallback-Index`; loggas som `complete_fallback`. Avbruten klient
  (`ctx.Err()`) stoppar kedjan — ingen pengar på svar ingen läser. Alla
  misslyckas → sista felets status/kod.
- `history.Handle` skickar även misslyckade attempts till `fillAttempt`, som
  nollar `cost_usd` för requesten; ett senare lyckat fallback-försök skriver
  in verklig kostnad (eller behåller estimatet när usage saknas).

## Tester

- `fallback_complete_test.go`: primär 429 → fallback svarar 200 med rätt
  header + fallback-index; alla misslyckas → sista felets kod; kandidatlistans
  ordning/dedupe/adapter-saknas.
- `failed_attempt_cost_test.go`: misslyckad request → 0 kostnad; misslyckad +
  lyckad fallback → bara verklig kostnad räknas.

## Not

Demo-chatten streamar och hade redan fallback; API-/agent-/CLI-anrop
(routerctl, MCP-drivna agenter) fick det först nu. Spend-trackern i minnet
gate:ade redan på `Success` — det var den durabla historiken (som dashboarden
läser) som saknade gaten.
