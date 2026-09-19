# ISSUE-085: Admin-hanterade demo-prompts (DB) för delbar CISO-demo

## Labels
- `epic: EPIC-15`
- `priority: P1`
- `type: feature`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Mål

Gör snabbknapparna i live-demon (`/chat`) **administrerbara och persistenta** i
DB, seedade med ett set som verkligen visar plattformens värde — så en demo-länk
kan skickas till en CISO som loggar in, klickar sig igenom värdet, och sedan tar
ett samtal. Ägarens idé efter PII→lokal-bytet ("det där var lysande").

## Levererat

- `internal/server/quickprompts.go`: `QuickPrompt{Label,Tier,Text,Note}`,
  DB-persistens i KV (`demo_quick_prompts`), `SeedQuickPrompts` (idempotent,
  körs vid start), `LoadQuickPrompts` (fallback till defaults),
  `QuickPromptsHandler` (JSON till chatten).
- Kurerad standarduppsättning som visar: billig routing (kostnad), **PII →
  lokal** (syntetiskt personnummer), premium för svårt, **fail-closed** på
  säkerhetsgranskning, balanserad för NIS2-fråga.
- `internal/server/promptspage.go`: admin-sida `/router/prompts` (lägg till,
  ta bort, återställ till standard); nav-item "💡 Demo-prompts".
- Demo-chatten laddar prompts från `/chat/quickprompts` (fallback inbyggd) och
  visar `Note` som tooltip.

## Acceptanskriterier (uppfyllda)

- Prompts seedas i DB vid start och överlever omstart; redigeringar skrivs över
  inte av seed.
- Standarduppsättningen innehåller PII→lokal-showcasen.
- Skrivskyddat läge utan data dir (visar defaults).
- Tester: seed/load/round-trip, JSON-handler, add/delete, validering, read-only.
