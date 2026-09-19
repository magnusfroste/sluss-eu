# ISSUE-066: Intent-aware classification — don't escalate doc/summarize to security_review on topic keywords alone

## Labels
- `epic: EPIC-11`
- `priority: P2`
- `type: enhancement`
- `sprint: post-beta`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Sluta över-tiera dokumentations-/sammanfattningsförfrågningar till `security_review` (premium) enbart för att texten *nämner* säkerhetstermer. Skilj **avsikt** (dokumentera/sammanfatta/förklara) från **ämne** (säkerhetskod). Detta är den första datadrivna closed-loop-förbättringen under EPIC-11 (se ISSUE-065).

## Bakgrund — observerat under dogfooding

Tokenizer kördes som backend för en VS Code-kodassistent (Continue). En **"uppdatera dokumentationen"**-förfrågan klassades som `security_review` + `risk=critical` → premium (~$0.10/anrop), eftersom Continue skickar med kodbasen som kontext och tokenizer-repot är fullt av säkerhetstermer.

Reproducerat via `/router/decision`:

| Prompt | task_type | modell |
|---|---|---|
| "update the README quickstart with the new make targets" | `simple_chat` | cheap-general ✅ |
| "update the documentation for the **security review** and **audit log** and **secret masking** modules" | `security_review` | premium ❌ |
| "document this function that scans for **vulnerability** and **exploit** and **cve** in auth tokens" | `security_review` | premium ❌ |

## Grundorsak

`hasExplicitSecurityReviewSignal` i `internal/classifier/task.go` (rad ~187) eskalerar på **bara omnämnande** av topp-nyckelord:

```go
containsAnyTerm(ctx.lower, []string{"xss","csrf","ssrf","vulnerability","security review","security audit","threat model","exploit","cve"})
```

Det finns ingen koppling till en faktisk säkerhets-*handling* (review/audit/scan/assess). En prompt vars verb är "document"/"summarize"/"explain" men som nämner ett säkerhetsämne trippar ändå regeln.

## Lösning — två lager

### 1. Omedelbar mitigering via policy (hot-reload, ingen deploy)
Lägg en regel i runtime-policyn: när `task_type == security_review` **och** prompten har tydlig doc-/summarize-intent (`contains_any: [document, documentation, summarize, summary, explain, describe, readme, changelog]`), nedgradera till `model_profile: balanced` (eller `summarization`-behandling) istället för premium. Demonstrerar closed-loop: produktion → policy → hot-reload, utan kodändring. Gatekeepas av regression-suiten.

### 2. Durabel fix i klassificeraren (root cause)
Inför **intent-medvetenhet**: `security_review` ska kräva en säkerhets-*handling* riktad mot innehållet (review/audit/scan/assess/pentest/"find vulnerabilities"), inte bara ämnes-substantiv. När en explicit doc-/summarize-/explain-intent finns och säkerhetssignalen *enbart* är topikal (substantiv-omnämnande utan handlingsverb), eskalera **inte** till `security_review` — låt den falla till `summarization`/`simple_*` enligt vanlig logik.

Bevara: en äkta begäran ("security review this login form for XSS", "audit the auth flow for vulnerabilities", "scan for injection") ska fortfarande bli `security_review`.

## Acceptanskriterier

- De tre fallen ovan: A förblir `simple_chat`; B och C blir **inte** `security_review` (utan `summarization`/balanced) när intenten är dokumentation.
- Äkta säkerhetsgranskningar förblir `security_review` (regressionsskydd — se golden cases nedan).
- Policy-mitigeringen (lager 1) är hot-reloadbar och täckt av `tests/policy-cases.yaml`.
- Klassificerar-ändringen (lager 2) har table-tester i `internal/classifier/task_test.go`.
- Hela regression-suiten (ISSUE-041) grön; inga nya felroutningar.

## Golden / regression-cases

Behåll som `security_review` (får INTE regrera):
- "security review this login form for XSS and secret leakage"
- "audit the auth flow for vulnerabilities"
- "scan the payment handler for injection"
- "threat model the new SSO integration"

Ska INTE längre vara `security_review` (doc-/summarize-intent + topikalt ämne):
- "update the documentation for the security review and audit log modules"
- "summarize the changes to the secret masking module"
- "explain what CSRF protection we have in the gateway"
- "write a changelog entry for the vulnerability scanner"

## Tekniska noter

- `internal/classifier/` måste förbli ren/regelbaserad (ingen LLM) och inom p95 < 20 ms.
- Klient-metadata är fortfarande inte betrodd; detta sänker *inte* risk för äkta säkerhetsarbete — det skiljer bara doc-intent från review-intent.
- Föredra lager 1 (policy) som omedelbar åtgärd och lager 2 (klassificerare) som durabel rot-fix; båda kan landa separat.

## Klar när

- Acceptanskriterierna uppfyllda, regression-suiten grön.
- Ett före/efter-mått visar att doc-frågor i ett säkerhetstungt repo inte längre premium-routas enbart på ämnes-keywords.

## Härkomst

Identifierat live under dogfooding (router bakom Continue). Första konkreta posten i closed-loop-slingan (ISSUE-065 / EPIC-11): produktionssignal → reproducerbart fall → policy/klassificerar-justering gated av regression-suiten.
