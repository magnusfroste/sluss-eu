# ISSUE-090 — MCP-verktyg för live-debug av en körande instans

- `epic: EPIC-06`
- `priority: P1`
- `type: backend`
- `sprint: 09`
- `state: done`

## Problem

När demons PII-väg 502:ade var upstream-felet bara synligt i flyktiga slog-loggar,
och man kunde inte avgöra "når routern DGX?" utan shell-access till EasyPanel.
MCP-ytan (ISSUE-074) var läs-only men saknade felinsyn och en reachability-probe.

## Lösning — tre tillägg till MCP-ytan

1. **`provider_probe`** — gör ett live-anrop mot en *konfigurerad* provider **från
   routerns eget nätverk** (`GET <base>/models` eller `chat` med en 1-token-
   completion) och returnerar status, latens, body-snutt och en **klarspråks-
   diagnos** (Cloudflare-felsida vs origin-5xx vs auth vs connect-fel). Endast
   konfigurerade providers (base_url från roster/registry) → ingen SSRF. URL:en
   härleds via `provider.ChatCompletionsURL` så probet träffar exakt vad ett
   riktigt anrop gör, oavsett base_url-form.
2. **`recent_errors`** — in-memory ring (`eventlog.ErrorRing`, handler på event-
   kön) av senaste misslyckade attempts: tid, provider, modell, error_code
   (`provider_5xx`/`provider_timeout`/...), attempt-index, latens. Ingen prompt-
   text. Ephemeral (nollas vid omstart — precis när man tittar).
3. **`get_roster`**: exponerar nu `reasoning_capable` per modell, så man kan
   bekräfta att en flagga faktiskt är *live i den körande registryn* (svarar
   "tog omstarten?").

## Verifiering

`errorring_test.go` (spelar bara in fel, ringbuffert-wrap, nil-säker),
`mcp_probe_test.go` (probe når stub → ok+status+diagnos, okänd provider →
fel, recent_errors returnerar ringens innehåll), tool-count 8→10. Full svit +
`make demo-ciso` grön.

## Non-goals

Persistent fel-logg (ringen är medvetet in-memory); godtyckliga URL:er i probe;
auto-remediering. Notis/alarm vid fel ligger i ISSUE-089-familjen.
