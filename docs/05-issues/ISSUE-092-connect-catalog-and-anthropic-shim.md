# ISSUE-092 — Connect-katalog (/connect) + Anthropic-kompatibel /v1/messages

- `epic: EPIC-15`
- `priority: P1`
- `type: backend`
- `sprint: 09`
- `state: done`

## Bakgrund (2026-07-09)

En model router ger noll värde tills klienter *pekar på den*. Aktiveringsytan är
minst lika viktig som landningssidan: devs måste kunna rikta sina IDE:er, agenter
och verktyg mot tokenizer. Två delar:

1. **/connect** — publik + instans-medveten katalog: per-klient-recept (base-URL
   + nyckel + `model=auto`). SEO/AEO (folk googlar "point Cursor at
   OpenAI-compatible base url") och in-product onboarding (visar instansens
   riktiga base-URL när `ROUTER_PUBLIC_URL` är satt).
2. **/v1/messages** — Anthropic Messages-kompatibel shim, så Anthropic-native
   klienter (Claude Code, Codex, Anthropic-SDK:er) kan peka på routern — inte
   bara OpenAI-wire.

## Implementation

- `internal/server/connect.go` — publik katalog (Cursor, Cline/Continue, Claude
  Code, Codex, Aider, OpenWebUI, AnythingLLM, LibreChat/Jan, OpenAI/Anthropic
  SDK). Mountas på `GET /connect` (publik), länkad från landningens CTA,
  sitemap, robots-allow, /llms.txt och admin-navet.
- `internal/server/anthropic.go` — översättnings-middleware runt chat-handlern:
  Messages-request → intern OpenAI-form → **samma** routing/policy/provider-väg →
  svar översatt tillbaka (JSON + SSE). Streaming översätter OpenAI-SSE till
  Anthropic-events (message_start / content_block_start / _delta / _stop /
  message_delta / message_stop). Fel → Anthropic error-envelope. `POST
  /v1/messages` med samma auth/scope som chat.

## Verifiering

anthropic: non-stream request+response-översättning, streaming-events, fel-
översättning, tom-messages-avvisning, content-block-text-extraktion. connect:
instans-base-URL renderas, placeholder utan `ROUTER_PUBLIC_URL`. Full svit +
`make demo-ciso` grön.

## Följd-bygge (2026-07-09) — tool-use + Anthropic /v1/models

- **Tool-use-passthrough**: Anthropic `tools` → OpenAI function-tools; `tool_use`
  (assistant) → OpenAI `tool_calls`; `tool_result` (user) → OpenAI `tool`-roll-
  meddelanden. Svar: OpenAI `tool_calls` → Anthropic `tool_use`-block (non-stream
  OCH streaming via `input_json_delta`), `stop_reason: tool_use`. `openai.Message`
  fick `tool_calls`/`tool_call_id`/`name` (omitempty). Gör Claude Code/Codex
  funktionella, inte bara text.
- **Anthropic `/v1/models`**: header-förhandling — `anthropic-version`-header →
  Anthropic-listformat (`{data:[{type:model,id,display_name}]}`), annars OpenAI-
  listan. Samma path, rätt form per klient.

## Non-goals (v1)

Images / extended thinking i /v1/messages. Modellval mappas till `auto` (routern
bestämmer) — klientens modellnamn är en hint.
