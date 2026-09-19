# ISSUE-111 — Agent capability gating: tool-risk-klassning + policyvillkor

- **Epic:** EPIC-13 (egress & PII)
- **Priority:** P1
- **State:** done (2026-08-01)
- **Bakgrund:** marknadssvepet `docs/00-product/14` — "agent gateway" blev en
  namngiven kategori mitt 2026 (Nutanix, Arcade, Manufact) och harness-tesen
  beskriver ordagrant vår arkitektur. Gapet vi identifierade: Sluss styrde
  **prompt→modell**, marknaden frågar nu även **agent→verktyg**.

## Problem

En agent-request deklarerar vilka verktyg modellen får anropa (`tools`). Sluss
såg bara `requires_tool_use: true` — ingen skillnad mellan `search_docs` och
`delete_account`. En CISO som släpper in agenter i produktion behöver kunna
säga *"agenter får läsa, men inte radera — och inte skicka data ut ur huset"*.

## Ändring

**Deterministisk tool-risk-klassificering** (`internal/classifier/tools.go`) —
samma filosofi som innehållsklassningen: nyckelordsregler, ingen LLM, rankad så
policy kan gate:a på högsta risk:

| Klass | Rank | Innebörd |
|---|---|---|
| `read` | 1 | Endast hämtning: search, get, list, fetch |
| `write` | 2 | Muterar internt tillstånd: create, update, upload |
| `external` | 3 | Lämnar organisationen: e-post, sms, publish, betalning |
| `destructive` | 4 | Radera, exekvera, admin — svårast att ångra |

- **Okänt verktyg → `write`**, aldrig `read`. Att anta läs-bara vore
  fail-open-valet.
- Båda wire-formaten stöds (OpenAI `{"function":{...}}` och Anthropic-plattform).
- **Endast namn** bärs vidare (max 32, dedupade) — aldrig verktygsargument,
  som kan innehålla kunddata. Namnen är evidens: *vilka förmågor låg på bordet*.
- Klassningen sker på **deklarationen**, före provideranropet, på fast path —
  så fail-closed-kontraktet håller: en prompt som kan nå ett destruktivt verktyg
  klassas som sådan **innan** någon modell ser den.

**JobDescriptor** får `tool_risk` + `tools_declared` (med i `SafeLogFields`,
dvs. audit/loggning).

**Policy DSL v1 utökas** med två villkor:
- `tool_risk: <klass>` eller `{in: [...]}` — validerat mot vokabulären vid
  parse, så en stavfel-regel inte tyst slutar matcha.
- `any_tool_matches: ["*payment*", "transfer_funds"]` — globmatchning mot
  deklarerade verktygsnamn, för att gate:a ett specifikt verktyg oavsett klass.

Exempel:

```yaml
- id: external_tools_blocked
  when: { tool_risk: external }
  route: { block: { code: tool_risk_external_denied, reason: agent may send data outside } }
- id: destructive_tools_stay_local
  when: { tool_risk: destructive }
  route: { constraints: { require_provider_tags: [local] } }
```

## Tester

- `tools_test.go` — rankning (högsta vinner), båda wire-formaten, okänt→write,
  namn-cap/dedupe.
- `toolrisk_test.go` — policy gate:ar: destructive→lokal, external→block,
  `any_tool_matches`→block; verktygslös prompt opåverkad; okänd `tool_risk`
  avvisas vid parse.
- `tool_job_test.go` — end-to-end genom `NewJobDescriptor`: destruktivt verktyg
  ger `tool_risk=destructive` i descriptorn och i `SafeLogFields`.

## Uppdatering 2026-08-01 (demo-beat)

NIS2-packet fick regel **4d** (`agent_high_capability_stays_local`,
`pv_nis2_baseline_2026_08`): `tool_risk in [external, destructive]` → require
`local`. Demo-manuset fick **Beat 9 — Agenten** med copy-paste-payload;
verifierad end-to-end i `TestDemoBeat9AgentPayload` (payload → destructive →
lokal; utan tools → fri routing).

## Utanför omfattning (nästa steg)

- Gate:a **svarets** `tool_calls` (vad modellen faktiskt begär) — kräver
  response-vägen och en policy för "avbryt anropet".
- Per-agent-identitet (non-human identity) och godkännandeflöden
  (human-in-the-loop innan destruktiva anrop).
- MCP-verktygsnivå: vilka MCP-servrar/verktyg en agent får nå genom Sluss.
- Konsol-UI för tool-risk-regler (går idag via YAML/pack; konsolreglerna
  täcker sensitivity/task, inte tool_risk än).
