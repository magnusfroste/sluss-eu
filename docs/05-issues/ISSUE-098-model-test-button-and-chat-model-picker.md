# ISSUE-098 — Per-model Test button + chat model picker (admin control panel)

- category: enhancement
- state: done
- epic: EPIC-15
- priority: P1
- type: backend|frontend
- sprint: 09
- klar: 2026-07-12

## Bakgrund

Två verkliga händelser i drift motiverade detta:

1. **Slug-felet** (`zai/glm-5.2` i stället för providerns egna `glm-5.2`)
   upptäcktes först som 400 i produktion och krävde MCP-felsökning. Regeln är:
   slugen ska vara exakt vad providerns egna `/models` returnerar — prefix som
   `zai/`, `dgx/` är OpenRouter-konventioner.
2. Chatten är **adminens kontrollpanel** (demo + systemverifiering) — men den
   kunde bara köra via klassificeraren. Att verifiera en specifik modell
   krävde curl.

## Levererat

### 1. Test-knapp per modellrad (Models-sidan)

- `POST /router/models/test` (admin-gated): 1-token completion med radens
  **exakta slug** mot dess provider, från routerns eget nät (samma
  SSRF-hållning som `provider_probe`: endast konfigurerade modeller).
- Inline ✅ + latens eller ❌ + HTTP-status; tooltip bär upstream-felet och en
  klarspråksdiagnos. 400/404 → slug-hintet ("exakt vad providerns /models
  returnerar").
- Reasoning-flaggade rader testar med `enable_thinking:false` → snabbt test
  som ändå bevisar slug + auth + endpoint.
- Funkar även för read-only-rader (registry-fallback utan roster).

### 2. Modellväljare i chatten (Auto vs pinned)

- `GET /chat/models` (admin-gated): routningsbara modeller med tier, egress
  (local/cloud) och ut-pris $/Mtok — kostnads-/CO₂-avvägningen syns i väljaren.
- Väljaren finns **endast på admin-sidan** — gästvyn (delade demolänkar) är
  orörd och kör alltid Auto.
- "Auto (router)" = default; att välja en modell skickar `model:"<id>"`
  (pinning). **Policyn gäller fortfarande** — en pinnad molnmodell + PII ger
  auditerad block (403), vilket i sig är demopoängen.
- Pinnade svar får en `pinned`-badge; hinten byter till "Pinned:
  classification is bypassed — policy is still enforced".

## Verifiering

`modeltest_test.go` (exakt slug skickas, 1 token, slug-diagnos på 400, 404 på
okänt id, knapp i sidan), `chatmodels_test.go` (tier/egress/pris, local-taggar,
nil-engine, picker admin-only). Full svit grön.

## Non-goals

- Ingen testknapp på provider-nivå (finns redan som `provider_probe` via MCP).
- Väljaren sparar inte valet mellan sidladdningar (medvetet: kontrollpanelen
  ska alltid starta i demo-läget Auto).
