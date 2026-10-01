# Issue index

Issues är skrivna som implementerbara tickets. Filnamn följer `ISSUE-XXX-title.md`.

## Triage snapshot 2026-05-19

Klart enligt kod- och test-evidens:

- ISSUE-001 till ISSUE-013.
- ISSUE-014 till ISSUE-019 — Classifier foundation: JobDescriptor, tokenestimat, feature extraction, task/riskregler och latency guard.
- ISSUE-020 till ISSUE-024 — Policy DSL v1, parser/validation, compiled policy cache, explanations och policy test runner.
- ISSUE-061 — Rebrand `tokenix` → `tokenizer`.
- ISSUE-062 — Context-processor pipeline (interface only).

Redo för agent:

- Inga öppna issues. Hela backlogen (ISSUE-001 till ISSUE-063) är `done` per
  2026-06-13 — se listan nedan.

Klart i sprint 05–08 (kod- och test-evidens):

- ISSUE-025 till ISSUE-041 — Routing/scoring/fallback (S05), observability (S06), evals/feedback (S07).
- ISSUE-042 — Secret masking v1.
- ISSUE-043 — Provider allow/deny per projekt.
- ISSUE-044 — Audit log (`internal/audit`: policy-reload, API-key-ändringar, blockerade requests).
- ISSUE-045 — Retention settings (`internal/retention`: per-tenant retention, prompt-logging-switch, cleanup-sweeper i `cmd/worker`).
- ISSUE-046 — API key scopes (`auth.RequireScope` per endpoint + `tenant.Tenant.HasScope`).
- ISSUE-056 — Beta release checklist (`07-operations/beta-release-checklist.md` med sign-off-process).
- ISSUE-047 — CLI för decision debug (`cmd/routerctl`).
- ISSUE-048 — SDK metadata helpers (`internal/sdk` + `examples/sdk-metadata`).
- ISSUE-049 — Spend simulator (`spend.Simulator`: baseline premium, besparing, riskjusterad besparing).
- ISSUE-050 — CI-integration för evals (dedikerade eval/policy-steg + `cmd/eval-report` artifact).
- ISSUE-051 — Budget caps (`internal/budget`: per tenant/projekt, warning, block/downgrade).
- ISSUE-052 — Route decision cache (`internal/decisioncache`: versionerad nyckel, endast låg-risk).
- ISSUE-060 — Global conservative mode (engine feature flag; osäkra tasks → ≥ balanced).
- ISSUE-053 — Model-specific prompt adapter skeleton (`internal/provider`: disabled-by-default system-role prompt mutator).
- ISSUE-054 — A/B policy simulation (`engine.DecisionComparison`, offline eval policy comparison, comparison artifacts).
- ISSUE-055 — Shadow routing (opt-in shadow policy decision, eventlog comparison payload, dashboard diff).
- ISSUE-057 — RBAC skeleton (`tenant.Role`, role middleware, admin-only dashboard).
- ISSUE-058 — Trained lightweight classifier experiment (offline-only dataset/training/baseline comparison; no production rollout).
- ISSUE-059 — Cost-quality frontier report (eval report frontier, deterministic recommendations).
- ISSUE-063 — Policy-gated context pipeline activation (`route.force.context_pipeline`, runtime policy cache, server-side activation).

Inga issues är markerade `needs-triage`, `needs-info`, `ready-for-human` eller `wontfix` efter denna pass.

## Spec detail pass 2026-05-19

High-risk open issues with expanded implementation contracts, acceptance criteria, verification notes, dependencies and non-goals:

- ISSUE-014 — `JobDescriptor` contract.
- ISSUE-015 — Fast token estimator.
- ISSUE-016 — Code-signal feature extraction.
- ISSUE-017 — Task classification rules.
- ISSUE-018 — Risk classification rules.
- ISSUE-020 — Policy DSL v1.
- ISSUE-021 — Policy parser and validation.
- ISSUE-022 — Compiled policy cache.
- ISSUE-025 — Candidate filtering.
- ISSUE-027 — Fallback planning.

## Rekommenderad prioritet

P0:

- ISSUE-001 till ISSUE-036.

P1:

- ISSUE-037 till ISSUE-056.

P2:

- ISSUE-057 och framåt.

## Tillagda via triage (post-sprint-1)

- ISSUE-061 — Rebrand `tokenix` → `tokenizer` (module path + product name). `type: refactor`, `state: done`, klar 2026-05-19.
- ISSUE-062 — Context-processor pipeline (interface only). `type: design`, `state: done`, klar 2026-05-19. Designval landade i ADR-0013.
- ISSUE-063 — Policy-gated context pipeline activation. `type: backend`, `state: done`, klar 2026-06-12. Tracks ADR-0013 tenant-policy opt-in before real processors ship.

## Sprint 09 — CISO pitch MVP (strategi 2026-07-05)

- ISSUE-075 — Tamper-evident audit-export (hash-kedja) + verify-CLI. `epic: EPIC-12`, `priority: P0`, `state: done`, klar 2026-07-05.
- ISSUE-076 — Compliance-rapportgenerator. `epic: EPIC-12`, `priority: P0`, `state: done`, klar 2026-07-05.
- ISSUE-077 — PII-detektor v1 (regelbaserad, fast path-säker). `epic: EPIC-13`, `priority: P0`, `state: done`, klar 2026-07-05.
- ISSUE-078 — Residens-/compliance-taggar i rostern + policyvillkor. `epic: EPIC-13`, `priority: P0`, `state: done`, klar 2026-07-05.
- ISSUE-079 — DB-backade API-nycklar v1 (mint/revoke, hash i vila). `epic: EPIC-14`, `priority: P0`, `state: done`, klar 2026-07-05.
- ISSUE-080 — Keys-adminsida. `epic: EPIC-14`, `priority: P1`, `state: done`, klar 2026-07-06.
- ISSUE-081 — Dashboard: "Egress & Compliance" + "Learning & routing". `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-06.
- ISSUE-082 — CISO-pitchdemo (scenario, dataset, körschema). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-06.
- ISSUE-084 — Residens-nekan → auditerad block (ej no_route). `epic: EPIC-13`, `priority: P2`, `state: done`, klar 2026-07-06 (#100).
- ISSUE-085 — Admin-hanterade demo-prompts (DB) för delbar CISO-demo. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-06.
- ISSUE-083 — Synligt routningsbyte i demo-chatten (moln → lokal). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-06. Signaturögonblicket (`.scratch/ciso-demo/signature-moment.md`).

- ISSUE-086 — Namngivna konsol-användare (accountability): DB-konton, PBKDF2, session-login, break-glass, per-user audit. `epic: EPIC-14`, `priority: P1`, `state: done`, klar 2026-07-07.
- ISSUE-087 — Delbar CISO-demolänk: token → least-privilege demo-session (chatt only), nav-fri vy, admin-hanterad + auditerad. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-08.
- ISSUE-088 — Reasoning-capable modeller: routern styr `enable_thinking` per anrop (svåra uppgifter tänker, enkla svarar direkt), höjd timeout, tänk-indikator + ren felruta i demon. `epic: EPIC-11`, `priority: P1`, `state: done`, klar 2026-07-09.
- ISSUE-089 — Namngivna demo-länkar + "vem har provat"-statistik (per-prospekt-token, opens/last-open, audit-attribution, MCP-label). `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-07-11. (Lead-formulär utgick → externt CRM, se ISSUE-091.)
- ISSUE-090 — MCP live-debug: `provider_probe` (reachability från routerns nät), `recent_errors` (in-memory upstream-fel), `reasoning_capable` i `get_roster`. `epic: EPIC-06`, `priority: P1`, `state: done`, klar 2026-07-09.
- ISSUE-091 — Agent-drivbara outreach-primitiv på MCP (`mint_key`, `key_usage`, `revoke_key`, `create_demo_link`, `demo_link_usage`) bakom `ROUTER_MCP_WRITE_ENABLED`. Tokenizer = create+measure, externt CRM = CRM/utskick. `epic: EPIC-06`, `priority: P2`, `state: done`, klar 2026-07-09.
- ISSUE-092 — Connect-katalog (`/connect`, publik + instans-medveten) + Anthropic-kompatibel `/v1/messages`-shim → peka valfri klient (Cursor, Claude Code, Codex, AnythingLLM, OpenWebUI, SDK:er) på routern. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-09.

## Labelstandard

- `category: enhancement|bug`
- `state: needs-triage|needs-info|ready-for-agent|ready-for-human|wontfix|done`
- `epic: EPIC-XX`
- `priority: P0|P1|P2`
- `type: backend|frontend|infra|security|data|product|test|refactor|design`
- `sprint: 00..09`

## Arbetsflöde

1. Skapa branch från issue.
2. Implementera.
3. Lägg till test.
4. Uppdatera dokumentation om kontrakt ändras.
5. Kontrollera latency om issue påverkar fast path.
6. Merge efter review.
- ISSUE-093 — CISO policy-konsol ("brandväggsregler") + rikare klassning. Komplett: `builtin:nis2-baseline`-mall + analys (2026-07-09); policy-konsol med dry-run (2026-07-10); rikare klassning financial/health/legal/security_classified (2026-07-12); regelredigering utan YAML med hot-activate/rollback (2026-07-12); provider-riskregister med kurerade taggar + fail-closed-gapvarning (2026-07-12). `epic: EPIC-12`, `priority: P1`, `state: done`, klar 2026-07-12.
- ISSUE-094 — Compliance-paket (moduler per marknad): regelverk-mallar levererade (`builtin:nis2-baseline/dora-baseline/gdpr-sovereign/pii-local` + Policy-lista); kvar rapportprofil + framing per regim. `epic: EPIC-12`, `priority: P1`, `state: ready-for-agent`. Se `00-product/10`. 2026-07-09.
- ISSUE-095 — Shadow AI gap-rapport (monitor mode): skuggpaket räknar vad som SKULLE hänt med känsliga prompts → `/router/gap-report` med siffran som säljer. `epic: EPIC-12`, `priority: P0`, `state: done`, klar 2026-07-10.
- ISSUE-097 — Engelska-svep över befintlig svensk app-text (admin, demo, rapporter, regime-profiler, gap-/kontrollrapport). `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-07-11.
- ISSUE-096 — Audited demo-data reset (nollställningen loggas i audit-kedjan) + retention synlig i policy-konsolen. `epic: EPIC-12`, `priority: P1`, `state: done`, klar 2026-07-10.
- ISSUE-103 — Rebrand Tokenizer → Sluss (sluss.eu): all användarriktad yta, README, deck; modulsökväg/repo-namn + DNS kvar som ägarbeslut. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-18.
- ISSUE-104 — Modulsökväg efter repo-rename: `github.com/magnusfroste/tokenizer` → `github.com/magnusfroste/sluss` (go.mod, 187 Go-filer, compose-imageref, Dockerfile-exempel). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-19.
- ISSUE-105 — Repo-struktur: dokumentationskorpusen (`00-product/` … `08-templates/`) flyttad under `docs/` som ett block; referenssvep i rot-md, Go-kommentarer och `.ai/tasks.json`; ren kod-först-rot. `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-07-19.
- ISSUE-121 — Synlig buildversion: commit + byggtid stämplas i imagen och visas i `/metrics` (`sluss_build_info`), konsolens sidfot, MCP `server_info` och startloggen — så en deploy som inte landat syns direkt. `epic: EPIC-14`, `priority: P2`, `state: done`, klar 2026-10-01.
- ISSUE-120 — Riskregistret på egen sida (`/router/risk`, egen menypost): leverantörsstyrning skild från anslutningar; Providers behåller fail-closed-varningen med länk; formulär- och tabellöverflöd rättade. `epic: EPIC-12`, `priority: P2`, `state: done`, klar 2026-10-01.
- ISSUE-119 — EU-färger på startsida, inloggning och logotyp (marinblå + EU-gul, IBM Plex/Instrument Serif som lokala typsnitt — inga Google Fonts från en datasuverän gateway). `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-10-01.
- ISSUE-118 — Instansens startsida: marknaden flyttad till www.sluss.eu; `/` visar produkten på en rad, Sign in, Connect och getting started; FAQ/JSON-LD/OpenGraph/sitemap/llms.txt borta; instansen indexeras inte (robots + noindex). `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-10-01.
- ISSUE-117 — UX: varför-rad per loggrad (klassning + matchande regler i nuvarande policy, prompten lagras aldrig), snabbfilter blocked/sensitive/local/cloud, local/cloud-bricka och Edit-rad på Models (tabellen får plats i 1280 px), hopfälld Advanced på dashboarden, läsbara belopp. `epic: EPIC-15`, `priority: P2`, `state: done`, klar 2026-10-01.
- ISSUE-116 — UX "where your data went": PII-kortbuggen (räknade bara blockerade) rättad, egress sparas per request och visas i loggen, dashboarden leder med local/cloud/blocked per känslighetsklass, policysidan visar aktiva regler i klartext med dry-run först och danger zone sist. `epic: EPIC-15`, `priority: P0`, `state: done`, klar 2026-09-30.
- ISSUE-115 — Roster hot-reload: ändringar på Models/Providers (modell, pris, tier, endpoint, compliance-taggar) gäller direkt — allt-eller-inget-byte av registry + adaptrar + omkompilerade policyer, fail-closed vid konflikt, auditerat (`roster.reload`); ny nyckel i env kräver fortfarande omstart. `epic: EPIC-06`, `priority: P1`, `state: done`, klar 2026-09-29.
- ISSUE-114 — UI för förstagångsbetraktaren inför demo: evidensrad före besparingen på dashboarden, sidomeny grupperad Evidence/Control/Show, känslighet + blockkod i request-loggen, konsolregler på agent-kapabilitet (`tool_risk`), Edit på providers, GitHub/AGPL på landningssidan. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-09-29.
- ISSUE-113 — Fallback-kedjan exekveras även för icke-streamande anrop (tidigare bara streaming; 429/5xx på primären gick rakt till klienten) + misslyckade attempts nollar pre-call-estimatet i historiken (dashboarden visade −8456 % vid död provider). Hittad vid demo-hälsokontroll. `epic: EPIC-06`, `priority: P0`, `state: done`, klar 2026-09-29.
- ISSUE-112 — Open source-cutover: en publik utvecklarrepo under AGPL-3.0; alla tredjeparts-identiteter/relationer avidentifierade eller borttagna, pitch-material och investerardeck till privat vault, allowlist-export riven → secret-scan i CI, LICENSE/SECURITY/CONTRIBUTING, cutover-runbook med ny historik. `epic: EPIC-15`, `priority: P0`, `state: done`, klar 2026-09-19.
- ISSUE-111 — Agent capability gating: deterministisk tool-risk-klassning (read/write/external/destructive, okänt→write) av deklarerade verktyg + policyvillkoren `tool_risk` och `any_tool_matches`; namn som evidens, aldrig argument. Stänger agent→verktyg-gapet från marknadssvepet (00-product/14). `epic: EPIC-13`, `priority: P1`, `state: done`, klar 2026-08-01.
- ISSUE-110 — Tidsfönster på MCP-verktyget `savings_report` (24h/72h/7d/30d, samma vokabulär som `incident_report`; `history.ByModelSince`, omräknad savings+green per period, fail-safe till all-time). Möjliggör agent-driven daglig CISO-brief. `epic: EPIC-12`, `priority: P2`, `state: done`, klar 2026-07-25.
- ISSUE-109 — Repo-split: allowlist-export (`scripts/publish-public.sh`) för publik open-core-kärna; fail-closed allowlist + path/content-guard + CI-steg; PUBLISHING.md-runbook; ADR-0014 (open core + provider-opt-in). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-25.
- ISSUE-108 — Persistenta konsol-sessioner: sessioner lagras i SQLite (ROUTER_DATA_DIR) och laddas vid boot → överlever redeploy (var in-memory, loggade ut alla vid omstart); write-through cache, revocable server-side; Secure-cookie bakom Cloudflare-tunnel via X-Forwarded-Proto. `epic: EPIC-14`, `priority: P1`, `state: done`, klar 2026-07-19.
- ISSUE-107 — Deterministisk savings-baseline: dyraste aktiverade premium-modellen väljs (inte map-ordning) och namnges i dashboard-widgeten ("vs all-premium (openai/gpt-4o)"); fixar −254 %-buggen live. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-19.
- ISSUE-106 — Docs-sanering efter pivoten: pre-pivot-produktdocs (00-product/01–07), MANIFEST och döda mallar raderade; CLAUDE/AGENTS + arkitektur-/engineering-/ops-docs korrigerade mot kodverifierat facit (SQLite-runtime, riktiga metrics/CI/CLI); ADR:er och historik orörda. `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-19.
- ISSUE-102 — Bilagor som policyklass: array-content accepteras (textdelar klassas som dokumenttext, bilddelar → requires_vision → NIS2-regel 4c lokal/fail-closed, original-payload bevaras till providern). `epic: EPIC-13`, `priority: P1`, `state: done`, klar 2026-07-18.
- ISSUE-101 — Landing: köparpositionering — risksektion (say yes on your rules), Why Tokenizer-USP:ar inkl. self-hosted, unlike-sats, CTA-fix (inget in i login-väggen; CRM = externt CRM). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-12.
- ISSUE-100 — Auditerad tenant-erasure (GDPR art. 17): radera per tenant (request-rader + spend + valfri nyckelåterkallelse), loggat i audit-kedjan. `epic: EPIC-12`, `priority: P1`, `state: done`, klar 2026-07-12.
- ISSUE-099 — Incident-evidens 24/72h-export: `/router/incident-report` + MCP `incident_report`, sensitivity/block_code i durabla loggen, evidenslänkar i policy-konsolen. `epic: EPIC-12`, `priority: P1`, `state: done`, klar 2026-07-12.
- ISSUE-098 — Test-knapp per modellrad (1-token test med exakt slug, slug-diagnos på 400/404) + modellväljare i chatten (Auto/pinned; policy gäller alltid; admin-only, gästvyn kör alltid Auto). `epic: EPIC-15`, `priority: P1`, `state: done`, klar 2026-07-12.
