# Beta release checklist

Denna checklista är gaten för att släppa Sluss till **beta**. Den är mer
omfattande än `release-checklist.md` (som gäller löpande releaser) och kräver
explicit sign-off per område innan beta öppnas för externa tenants.

> Använd kopiera-och-bocka-av per release. Varje rad ska vara verifierad med
> evidens (testkörning, dashboard-skärmdump, mätvärde eller länk), inte antagen.

## 1. Funktion

- [ ] `/v1/chat/completions` fungerar non-streaming.
- [ ] `/v1/chat/completions` fungerar streaming.
- [ ] `model: auto` routar utan klientangiven modell.
- [ ] Minst tre modellprofiler finns och är aktiva i registret.
- [ ] Policy kan tvinga cheap/balanced/premium.
- [ ] `/router/decision` dry-run returnerar beslut utan provideranrop.
- [ ] Response-headers sätts: `x-router-request-id`, `x-router-selected-model`,
      `x-router-policy-version`, `x-router-route-class`.

## 2. Latency

Mätt under representativ last, p95, **före** provideranrop.

- [ ] Feature extraction p95 < 20 ms.
- [ ] Policy eval p95 < 10 ms.
- [ ] Total router overhead p95 < 100 ms.
- [ ] Fast path gör inga LLM- eller externa anrop (verifierat i kod + trace).
- [ ] Latency-histogram syns i `/metrics` och på dashboarden.

> Mätmetod: `make load` (ISSUE-064) — concurrent lasttest mot mock-providern,
> rapporterar end-to-end p50/p95/p99 + felkvot och skrapar
> `router_routing_overhead_ms` från `/metrics`. Referensmätning (25 samtidiga ×
> 500 req): router-overhead p95 ≈ 0,05 ms, end-to-end p95 ≈ 12 ms, 0 fel.

## 3. Fallback & resiliens

- [ ] Fallback-kedjan byggs **före** första provideranropet.
- [ ] För risktunga tasks går fallback alltid uppåt (mer kapabel), aldrig nedåt.
- [ ] Provider timeout/5xx/rate-limit normaliseras till interna felkoder.
- [ ] Streaming-fallback sker före first token; efter first token krävs
      explicit klient-opt-in för restart.
- [ ] Provider health påverkar candidate-filtrering (osund provider exkluderas).
- [ ] Fallback rate och provider error rate syns på dashboarden.

## 4. Säkerhet & integritet

- [ ] API-nycklar lagras hashade (sha256), aldrig i klartext.
- [ ] Provider-secrets lagras säkert och läcker inte i loggar.
- [ ] Secret masking aktiv på alla utgående fel-/loggränser (ISSUE-042).
- [ ] Provider allow/deny per projekt enforced, även för pinnade modeller (ISSUE-043).
- [ ] Audit log skriver policy-ändringar, API-key-ändringar och blockerade
      requests (ISSUE-044).
- [ ] Prompt logging är **av** som default och kan stängas av per tenant (ISSUE-045).
- [ ] Retention per tenant konfigurerad; cleanup-job kört minst en gång (ISSUE-045).
- [ ] API key scopes enforced per endpoint (`403 insufficient_scope`) (ISSUE-046).
- [ ] Policy kan blockera provider/modell och felmeddelandet röjer ingen prompt.

## 5. Observability

- [ ] Decision log skrivs (async, blockerar inte request-path).
- [ ] Attempt log skrivs per provideranrop (primär + fallback).
- [ ] Cost- och latency-metrics exponeras på `/metrics`.
- [ ] Provider health-tracker uppdateras.
- [ ] Event queue backlog och drop-räknare övervakas.
- [ ] Spend aggregation uppdateras och syns på dashboarden.

## 6. Evals & kvalitet

- [ ] Minst 50 evalfall i datasetet.
- [ ] Policy golden-cases passerar.
- [ ] Eval smoke körs i CI.
- [ ] Regression-suite för felrouting passerar.
- [ ] `make test` grön (unit + integration).
- [ ] `make lint` ren.

## 7. Operativ beredskap

- [ ] SQLite-schema-migrationer (idempotenta ALTER vid boot) verifierade mot en kopia av produktionsdatafilen.
- [ ] Registry validerat mot aktiv version.
- [ ] Provider credentials verifierade i målmiljön.
- [ ] Rollback-plan finns (kod, policy, registry — se `release-checklist.md`).
- [ ] Runbook och incident-response uppdaterade (`07-operations/`).
- [ ] SLO/SLA-mål dokumenterade och larm konfigurerade (`slo-sla.md`).
- [ ] Dashboard visar staging-/canary-data.

## Sign-off

Beta öppnas först när **alla** områden är gröna och varje ägare har signerat.
Ingen enskild person signerar sitt eget och ett annat kritiskt område.

| Område | Ägare (roll) | Status | Signatur | Datum |
|---|---|---|---|---|
| Funktion | Tech lead | ☐ | | |
| Latency | Performance owner | ☐ | | |
| Fallback & resiliens | Routing owner | ☐ | | |
| Säkerhet & integritet | Security owner | ☐ | | |
| Observability | SRE/On-call | ☐ | | |
| Evals & kvalitet | Quality owner | ☐ | | |
| Operativ beredskap | Release manager | ☐ | | |

**Go/No-Go-beslut:** _Release manager_ sammanställer signaturerna och fattar
ett dokumenterat Go/No-Go. Vid No-Go listas blockerande punkter med ägare och
plan. Beslutet (med datum, version och deltagare) loggas i `DECISION_LOG.md`.

### Process

1. Skapa en kopia av denna checklista per beta-kandidat (t.ex. i release-PR:n).
2. Verifiera varje rad med evidens; länka testkörning/skärmdump/mätvärde.
3. Områdesägare signerar sitt område.
4. Release manager fattar Go/No-Go och loggar beslutet.
5. Efter release: kör efter-release-kontrollerna i `release-checklist.md`.
