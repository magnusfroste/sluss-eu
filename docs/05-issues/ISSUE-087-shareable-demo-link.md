# ISSUE-087 — Delbar CISO-demolänk (token → demo-only session)

- `epic: EPIC-15`
- `priority: P1`
- `type: frontend`
- `sprint: 09`
- `state: done`

## Problem

En CISO ska kunna öppna live-demon via en länk och klicka sig igenom värdet —
utan att få ett admin-lösenord. Tidigare låg `/chat` bakom admin-guarden, så en
delad länk krävde admin-inloggning och exponerade hela konsolen.

## Lösning

En hemlig, admin-genererad token ger en länk `<public-url>/demo?token=<token>`
som startar en **least-privilege demo-session** (roll `demo`): endast
`/chat`-rutterna, aldrig en admin-sida.

- `GET /demo?token=` validerar token (constant-time) mot en KV-lagrad hemlighet
  och startar en demo-session; fel/avstängd länk → `/router/login` (fail-closed).
- `AdminAuth.RequireDemo` släpper in demo-roll på chatt-rutterna; `Require`
  avvisar uttryckligen demo-roll från admin-sidor (redirect till `/chat`, 403 på
  icke-GET).
- Demo-gästen får en **nav-fri** chattvy (fokuserad produktdemo, inte konsolen).
- Token hanteras på Demo-prompts-sidan: skapa / rotera / stäng av. Full URL
  byggs från `ROUTER_PUBLIC_URL`.
- Varje öppning och varje rotation/avstängning loggas i audit-kedjan.

## Verifiering

`internal/server/demolink_test.go`: giltig token → demo-only access; demo-session
avvisas från admin (redirect + 403 på POST); fel/avstängd token sätter ingen
cookie; URL-byggaren. Full svit + `make demo-ciso` grön.

## Non-goals

Per-gäst-token/utgång, rate limiting, CAPTCHA. Länken är för en riktad pitch och
kan roteras/stängas av direkt; live-anrop kostar (delas medvetet med en känd
mottagare).
