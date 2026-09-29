# ISSUE-115 — Modell- och provideränringar gäller utan omstart (roster hot-reload)

- **Epic:** EPIC-06 (provider execution) / EPIC-14 (admin)
- **Priority:** P1
- **State:** done (2026-09-29)

## Bakgrund

Under demoförberedelserna krävde varje ändring på Models/Providers-sidan
(ny modell, pris, tier, endpoint, compliance-taggar) en omstart i EasyPanel —
tre omstarter på en kväll. Orsak: registret (modellkatalogen) och en HTTP-
adapter per provider byggdes en gång vid start; sidorna skrev bara till
SQLite. Policyn däremot var redan hot-reloadbar. CLAUDE.md-invarianten
"policy and registry can be reloaded without a code deployment" höll alltså
bara till hälften.

## Ändring

- **En byggväg.** Startkoden i `cmd/router` blev `buildRoster`: läser
  rostern ur SQLite, bygger registry-definition + adapter per provider med
  nyckel i miljön, applicerar tier-overrides och den vid start cachade
  OpenRouter-prislistan (ingen utgående call på admin-vägen). Samma funktion
  används vid start och vid omladdning.
- **`RosterReloader`** (`internal/server/rosterreload.go`): allt-eller-inget.
  1. bygg och validera ny snapshot,
  2. kompilera om *varje* aktiv policy (live, shadow, experiment) mot den
     (`policy.Cache.Rebind`, tvåfas — inget ändras förrän commit),
  3. först när allt lyckats: byt registry, adapter-set och policycacher.
  Ett fel lämnar routingen exakt som den var (fail-closed även för
  konfiguration), t.ex. om en borttagen modell tvingas av aktiv policy.
- **Varje försök auditeras** (`roster.reload`, aktör, registry-version,
  antal modeller, ev. providers utan nyckel; misslyckade med orsak).
- **Chat-vägen** läser adapter-setet en gång per request via en atomär
  pekare — en request blandar aldrig två rostrar; request-vägen tar aldrig
  reload-låset.
- **Sidorna** säger nu "saved — live now (N models, registry-roster-…)"
  eller "saved but NOT applied — routing is unchanged: <orsak>". Texterna
  "apply on restart" blir "apply immediately" när reload är aktiv.

## Kvar som kräver omstart (medvetet)

- **Ny nyckel** i miljön: processen läser env vid start. Reload säger det
  explicit ("no key in the running process for X — set it and restart").
- **Mock-läge** (ingen `OPENROUTER_API_KEY`): registret är inte roster-drivet.
- **Dashboardens premium-baseline och energiestimat** räknas vid start —
  besparingsprocenten mot "all-premium" uppdateras vid nästa omstart.

## Tester

- `TestCacheRebindTwoPhase` — avvisar roster som tappar en tvingad modell,
  inget ändras före commit.
- `TestRosterReloaderSwapsAtomicallyOrNotAtAll` (`-race`, samtidiga läsare) —
  lyckat byte, två avvisade (policy-konflikt, byggfel), audit 1 ok / 2 fail.
- `TestApplyRosterEditMessages` — restart/live/NOT applied-meddelanden.
- **End-to-end** mot riktig router i roster-läge: PII 403 → lägg till
  local-taggad provider + modell via UI → PII routas lokalt (200,
  `X-Router-Egress: local`) → ta bort taggen → 403 igen. Ingen omstart.
