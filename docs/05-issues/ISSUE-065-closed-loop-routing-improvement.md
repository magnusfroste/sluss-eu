# ISSUE-065: Closed-loop routing improvement (production feedback → policy tuning)

## Labels
- `epic: EPIC-11`
- `priority: P2`
- `type: enhancement`
- `sprint: post-beta`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Stäng återkopplingsslingan: använd signaler från verklig användning för att *systematiskt* föreslå policy-justeringar som förbättrar routningens kvalitet/kostnad — utan att offra förklarbarhet eller införa modelldrift. Förbättringar shippas som **policy** (hot-reload), inte som omträning.

## Bakgrund

Mycket av infrastrukturen finns redan:
- **Outcomes-store** (ISSUE-039): `Record()` + `Acceptance()` per task_type.
- **Shadow-routing + comparison** (ISSUE-055): event-loggen bär redan `shadow_selected_model` och `shadow_cost_delta_microusd` per request → kontrafaktisk kostnad gratis.
- **Rik per-request-logg**: task_type, tier, kostnad, latens, error_code, tokens, first_token_ms.
- **eval-report** + **regression-suite** (ISSUE-041): offline-validering.
- **Hot-reload av policy**: förbättringar kan rullas utan deploy.

Det som saknas är **kvalitetssignalen från produktion**. Kostnad och latens är objektiva och loggas; "var svaret bra?" gör det inte — och klienter (t.ex. Continue) skickar ingen explicit acceptans. Detta issue fyller den luckan och kopplar ihop delarna.

## Acceptanskriterier

1. **Fånga proxy-kvalitetssignaler vid gatewayen** (inga klientändringar):
   - Retry-/regenererings-detektion: samma tenant + nära-identisk prompt inom ett kort fönster → tidigare svar otillräckligt (negativ signal). Loggas, läcker ingen prompt-text (jfr secret-masking).
2. **Aggregerad analys** per `(task_class, vald tier)`: retry-rate, fel-rate, latens, kostnad, samt shadow-delta ("hade en högre tier ändrat valet?"). Utökar gärna `eval-report` / dashboarden.
3. **Policy-förslag, human-in-the-loop**: rapporten föreslår konkreta tröskeljusteringar (höj/sänk min-tier per klass) **med regression-suite-impact** — den föreslår, en människa godkänner, sedan hot-reload. Ingen full-auto i v1.
4. **Tester**: enhetstester för retry-detektion och förslagslogiken; regression-suiten gatekeepar varje föreslagen policyändring.

## Tekniska noter

- Bevara fast path-latencybudgeten — signalfångst får inte ligga i request-vägen (async, som event-loggen).
- Ingen prompt-text eller secret i loggade signaler; återanvänd `internal/secrets`-maskering.
- Börja suggestion-baserat; auto-justering från brusiga signaler riskerar oscillation och kostnadsrusning.
- Bygg på befintligt: `internal/outcomes`, shadow-fälten i `internal/eventlog`, `cmd/eval-report`, regression-suiten.

## Klar när

- Acceptanskriterierna är uppfyllda och testerna passerar.
- En körning producerar minst ett regression-validerat policy-förslag från inspelad trafik.
- Dokumentation/runbook beskriver hur ett förslag granskas och hot-reloadas.

## Not

Detta är seed till en post-beta-epic **EPIC-11 "Closed-loop routing improvement"**. Identifierat under lokal dogfooding (router som backend för en VS Code-kodassistent), där shadow-premium redan ger kontrafaktiska kostnadsdeltan per request.
