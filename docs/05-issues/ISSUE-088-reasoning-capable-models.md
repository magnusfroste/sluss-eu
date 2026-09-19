# ISSUE-088 — Reasoning-capable modeller: routern styr thinking per anrop

- `epic: EPIC-11`
- `priority: P1`
- `type: backend`
- `sprint: 09`
- `state: done`

## Problem

Ägarens enda lokala modell (qwen36-27b på DGX/vLLM) är en reasoning-modell:
bäst generellt, men tänker 60–80 s före svaret. I demon såg PII→lokal-vägen
frusen ut (tom bubbla), klipptes av timeout (30–60 s) och kunde ge en rå
Cloudflare-502-sida i chatten. Att byta till en sämre non-thinking-modell vore
fel avvägning — hårdvaran kör bara en modell åt gången.

## Lösning

En modell kan markeras **`reasoning_capable`** (Models-sidan → "tänkande").
För sådana modeller sätter routern vLLM:s `chat_template_kwargs.enable_thinking`
per anrop från jobbets `requires_reasoning`-signal:

- Sammanfattning / PII-svar / enkelt → **thinking AV** → svar direkt.
- Svår debugging / säkerhet → **thinking PÅ** → full kvalitet.

Samma modell, deterministiskt styrd — i linje med "ingen LLM i routingbeslutet".
Verifierat live mot DGX: `enable_thinking:false` ⇒ inget tänk, `finish:stop`;
`/no_think`-suffix fungerar inte.

## Implementation

- `registry.Model.ReasoningCapable` + `providercfg.Model` + roster-kolumn
  `reasoning_capable` (idempotent migration) + kryssruta på Models-sidan (rad-
  och add-formulär — radformuläret måste bära fältet, annars nollas det vid
  re-tier).
- `openai.ChatRequest.ChatTemplateKwargs` (vLLM-extension; skickas ENDAST när
  routern satt direktivet, så OpenAI/OpenRouter aldrig ser fältet).
- `NormalizedModelRequest.EnableThinking *bool` + `TimeoutHint` (Clone/ToOpenAI).
- Adaptern: `TimeoutHint` vinner över default-timeout OCH lyfter den hårda
  `http.Client.Timeout` (klienten kopieras med Timeout=0; delad Transport
  behålls). Reasoning-modeller får 120 s (långsam on-prem-GPU, ~10 tok/s).
- chat.go: `reasoningDirective(modelID, job)` appliceras på Complete-vägen och
  per streaming-kandidat (fallback-kedjan kan blanda modelltyper).
- Demo-chatten: "🧠 tänker…"-indikator medan `delta.reasoning` strömmar; ren
  felruta vid !res.ok (aldrig rå proxy-HTML).

## Verifiering

`TestReasoningDirectiveFollowsJobSignal`, `TestToOpenAIChatTemplateKwargs`,
`TestCloneCopiesThinkingAndTimeout`, `TestOpenAIAdapterSendsThinkingDirective`
(wire + timeout-lyft), roster-roundtrip, `TestRegistryEntriesCarryReasoningCapable`.
Full svit + `make demo-ciso` grön.

## Non-goals

Att visa själva reasoning-texten i UI (bara indikator); per-task
timeout-konfiguration; andra reasoning-dialekter än vLLM/Qwen3 (fältet är
generiskt nog att utöka).
