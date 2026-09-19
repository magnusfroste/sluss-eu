# Routing policy reference — Policy DSL v1

Detta dokument definierar **Policy DSL v1**, det kontrakt som senare parser (ISSUE-021),
validator, compiled-policy-cache (ISSUE-022), explanations (ISSUE-023) och policy
test runner (ISSUE-024) implementerar mot. Dokumentet är källan till sanning för
schemat. Det innehåller ingen produktkod.

DSL:en är deterministisk och kompilerbar. Policyutvärdering körs på fast path och
får inte göra LLM-anrop, externa I/O eller andra runtime-beroenden före provider
call. Policy refererar till **interna modellprofiler** och capabilities, inte till
provider-marknadsnamn.

## 1. Format

Policy uttrycks som YAML. Strukturen ska vara serialiserbar som ekvivalent JSON,
dvs. utan YAML-specifika features som anchors, aliases eller custom tags. Allt
ska kunna uttryckas med skalärer, kartor och listor.

```yaml
version: pv_2026_05_19
metadata:
  owner: platform
  description: Default routing policy
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: example_rule
    when: {}
    route: {}
```

Samma policy i JSON är giltig och har identisk semantik:

```json
{
  "version": "pv_2026_05_19",
  "metadata": {"owner": "platform", "description": "Default routing policy"},
  "settings": {
    "default_model_profile": "balanced",
    "conservative_unknowns": true,
    "max_router_overhead_ms": 100,
    "default_timeout_ms": 30000,
    "default_retention": "standard"
  },
  "rules": [
    {"id": "example_rule", "when": {}, "route": {}}
  ]
}
```

## 2. Top-level-fält

| Fält       | Obligatoriskt | Beskrivning                                                                                                                  |
|------------|---------------|------------------------------------------------------------------------------------------------------------------------------|
| `version`  | Ja            | Stabil policyversion, t.ex. `pv_2026_05_19`. Loggas, ingår i `RouteDecision` och i debug-/explanation-output.                |
| `metadata` | Nej           | Dokumentationsyta. V1 beskriver minst `owner` och `description` som fria strängar. Övriga nycklar tillåtna men osemantiska.  |
| `settings` | Ja            | Defaults och säkerhetsinställningar. Se §3.                                                                                  |
| `rules`    | Ja            | Ordnad lista av regler. Filordning är semantisk. Se §4–§6.                                                                   |

## 3. `settings`

`settings` definierar policy-wide defaults och säkerhetsbeteende. Alla fält är
obligatoriska i v1:

| Fält                      | Typ       | Beskrivning                                                                                                                                                |
|---------------------------|-----------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `default_model_profile`   | enum      | Profilen som väljs när ingen regel styr hårdare. V1 tillåter `cheap`, `balanced`, `premium`.                                                               |
| `conservative_unknowns`   | bool      | När `true` får okänd eller låg-confidence klassificering aldrig sänka säkerhetsnivå. Okänd task/risk routas högst till `balanced` eller `premium`.         |
| `max_router_overhead_ms`  | int       | Budget för routerarbete före provider call. V1 default är `100`.                                                                                           |
| `default_timeout_ms`      | int       | Fallback-timeout när varken regel eller modellprofil sätter timeout.                                                                                       |
| `default_retention`       | enum      | Policy-default för retention. V1: `standard` eller `none`.                                                                                                 |

## 4. Regelstruktur

```yaml
rules:
  - id: auth_requires_premium
    description: Auth code requires premium reasoning and verifier
    when:
      any_file_matches:
        - "**/auth/**"
        - "**/*auth*"
      risk_level:
        in: [high, critical]
    route:
      force:
        model_profile: premium
        verifier: true
      constraints:
        require_capabilities: [tool_use, json_schema]
      reason: Auth-related files are high risk
```

| Fält          | Obligatoriskt | Beskrivning                                                                                                                                                          |
|---------------|---------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `id`          | Ja            | Stabil, unik inom policy. Snake_case rekommenderas. Används i logging och explanations.                                                                              |
| `description` | Nej           | Mänsklig beskrivning. Får loggas i explanations.                                                                                                                     |
| `when`        | Ja            | Matchobjekt. Tomt objekt (`{}`) matchar alltid och används för explicit defaultregel. Se §5.                                                                         |
| `route`       | Ja            | Route-objekt med minst en av `block`, `force`, `constraints`, `defaults` eller en route-hint. Se §6.                                                                 |
| `reason`      | Nej           | Förklaringssträng som infogas i decision explanation när regeln matchar (kan ligga på `route`-nivå för v1-bakåtkompatibilitet med befintlig referens).               |

## 5. `when` — matchvillkor

Flera fält i samma `when` tolkas som **logiskt AND**. Listor inom `in:` tolkas som
**logiskt OR** över de listade värdena. Enum-fält accepterar både kort form
(`task_type: trivial_git`) och `{ in: [...] }`-form.

| Villkor                 | Typ                                              | Semantik                                                                                                       |
|-------------------------|--------------------------------------------------|----------------------------------------------------------------------------------------------------------------|
| `task_type`             | sträng eller `{ in: [...] }`                     | Matchar `JobDescriptor.task_type` mot v1-vokabulären (§7).                                                     |
| `risk_level`            | sträng eller `{ in: [...] }`                     | Matchar `JobDescriptor.risk_level` mot v1-vokabulären (§7).                                                    |
| `tenant`                | sträng eller `{ in: [...] }`                     | Matchar verifierad `tenant_id` från auth-kontext, inte klient-hints.                                           |
| `project`               | sträng eller `{ in: [...] }`                     | Matchar verifierat `project_id` från auth-kontext, inte klient-hints.                                          |
| `prompt_tokens_gt`      | int                                              | Sann när `JobDescriptor.prompt_tokens_estimate > N`.                                                           |
| `prompt_tokens_lt`      | int                                              | Sann när `JobDescriptor.prompt_tokens_estimate < N`.                                                           |
| `contains_any`          | lista av strängar                                | Case-insensitive substring-match mot normaliserade prompt-/metadata-signaler (parser definierar normalisering).|
| `any_file_matches`      | lista av globmönster                             | Glob-match mot `JobDescriptor.files_touched`. Stöder `**`, `*` och `?`.                                        |
| `requires_tool_use`     | bool                                             | Matchar `JobDescriptor.requires_tool_use`.                                                                     |
| `requires_json_schema`  | bool                                             | Matchar `JobDescriptor.requires_json_schema`.                                                                  |
| `requires_vision`       | bool                                             | Matchar `JobDescriptor.requires_vision`.                                                                       |
| `tool_risk`             | sträng eller `{ in: [...] }`                     | Matchar `JobDescriptor.tool_risk` — högsta klassen bland deklarerade verktyg (`read` < `write` < `external` < `destructive`). Okänt verktyg klassas `write` (fail-safe). |
| `any_tool_matches`      | lista av globmönster                             | Glob-match mot `JobDescriptor.tools_declared` (deklarerade verktygsnamn). Stöder `**`, `*`, `?`.                |
| `sensitivity`           | sträng eller `{ in: [...] }`                     | Matchar `JobDescriptor.sensitivity` (`none`, `source_code`, `legal`, `financial`, `health`, `security_classified`, `pii`, `secrets_possible`).                        |
| `router_mode`           | enum                                             | En av `auto`, `cheap`, `balanced`, `premium`, `disabled`.                                                      |

Klient-tillhandahållna hints (`task_type_hint`, `risk_level_hint`, `sensitivity_hint`,
`tenant_id_hint`, `project_id_hint`) får **inte** matchas av `when` i v1. `when`
ser endast verifierade descriptor-fält. Policy som vill bemöta hints får göra det
indirekt via `conservative_unknowns` eller via `task_type: unknown_high_risk`.

## 6. `route` — handlingar

Varje `route` ska innehålla minst ett av `block`, `force`, `constraints`,
`defaults` eller en route-hint. Semantiken är:

### 6.1 `block`

Stoppar requesten innan provider call. Kan vara `true` (block utan detaljer) eller
ett objekt:

```yaml
route:
  block:
    code: router_disabled
    reason: Router mode disabled by policy
    status: 403   # valfri HTTP-status, default 451
```

Block har högst prioritet och avslutar policyutvärderingen.

### 6.2 `force`

Tvingar val eller miniminivå som senare scoring **inte får sänka**. V1-fält:

| Fält                  | Typ    | Semantik                                                                                                            |
|-----------------------|--------|---------------------------------------------------------------------------------------------------------------------|
| `model_profile`       | enum   | Bred profil: `cheap`, `balanced`, `premium`.                                                                        |
| `model_profile_name`  | sträng | Intern registry-profil, t.ex. `cheap-general`, `balanced-coder`, `premium-reasoning`.                               |
| `provider`            | sträng | Intern provideridentifierare (registry-namn, inte marknadsnamn).                                                    |
| `model`               | sträng | Intern modellidentifierare. Får användas i tenantpolicy, undviks i default-policy.                                  |
| `verifier`            | bool   | Tvinga verifier-pass.                                                                                               |
| `timeout_ms`          | int    | Tvingad timeout som inte får överskridas.                                                                           |
| `retention`           | enum   | `standard` eller `none`.                                                                                            |

### 6.3 `constraints`

Begränsar kandidatsetet **innan** scoring. Ackumulerar över matchade regler.

| Fält                       | Typ                 | Semantik                                                                            |
|----------------------------|---------------------|-------------------------------------------------------------------------------------|
| `allowed_providers`        | lista av strängar   | Whitelist. Snitt över alla matchande regler.                                        |
| `denied_providers`         | lista av strängar   | Blacklist. Union över alla matchande regler.                                        |
| `allowed_models`           | lista av strängar   | Whitelist på interna modellnamn.                                                    |
| `denied_models`            | lista av strängar   | Blacklist på interna modellnamn.                                                    |
| `require_capabilities`     | lista av capability | Union. Alla listade capabilities måste finnas i vald modell.                        |
| `deny_capabilities`        | lista av capability | Union. Modeller med dessa capabilities filtreras bort.                              |
| `max_cost_usd`             | float               | Övre tak. Tas som minsta värde över matchande regler.                               |
| `max_latency_ms`           | int                 | Övre tak. Tas som minsta värde över matchande regler.                               |
| `retention`                | enum                | Strängare värde vinner (`none` > `standard`).                                       |
| `fallback_model_profiles`  | lista av profiler   | Föreslagen fallback-ordning. Måste konstrueras före första provider call.           |
| `require_provider_tags`    | lista av strängar   | Residens/compliance (ISSUE-078). Kandidaten måste bära **alla** taggar (modellens effektiva taggar = modell ∪ provider, satta i rostern). Normaliseras till gemener. |
| `deny_provider_tags`       | lista av strängar   | Kandidater som bär **någon** av taggarna filtreras bort.                            |

Tagg-villkoren prövas i samma enda constraint-kontroll som allow/deny-listorna
(`providerModelConstraintReason`) och gäller därför även **pinnade** modeller
(explicit klientmodell, `force.model`, `router_mode=disabled`) — en pin kan
aldrig kringgå ett residenskrav; den blockeras med 403. Ingen compliant
kandidat kvar ⇒ en **förstklassig, auditerad 403-block** med block-kod
`residency_no_compliant_provider` (ISSUE-084) — syns i auditkedjan,
blockeringsräknaren och kontrollrapporten, aldrig tyst fallback och aldrig ett
generiskt `no_route`. (Tom kandidatmängd av andra skäl ⇒ oförändrat `no_route`.)
Exempel: `when: {sensitivity: pii}` + `constraints: {require_provider_tags:
[eu-resident, dpa-signed]}`.

### 6.4 `defaults`

Sätter värden **endast** när ingen tidigare `block`, `force` eller `constraints`
har avgjort dem. Används för policy-defaults och för den explicita
defaultregeln. V1 accepterar samma fältuppsättning som `force` och `constraints`
men appliceras sist.

### 6.5 Route-hints (bakåtkompatibilitet)

För kompatibilitet med tidigare referens accepteras följande hints direkt under
`route`. De mappas av parser till `force`, `constraints` eller `defaults`:

| Hint               | Mappning                                              |
|--------------------|-------------------------------------------------------|
| `tier`             | → `defaults.model_profile` (om inte annars angivet)   |
| `model`            | → `force.model`                                       |
| `provider`         | → `force.provider`                                    |
| `fallback_tier`    | → `constraints.fallback_model_profiles: [<tier>]`     |
| `fallback_models`  | → `constraints.fallback_model_profiles` (resolvad)    |
| `verifier`         | → `force.verifier`                                    |
| `max_cost_usd`     | → `constraints.max_cost_usd`                          |
| `timeout_ms`       | → `force.timeout_ms` om `force` annars finns, annars `defaults.timeout_ms` |
| `retention`        | → `constraints.retention`                             |
| `require_capability` | → `constraints.require_capabilities: [<value>]`     |

Nya policys bör använda `force`/`constraints`/`defaults` direkt. Hints är ett
migrationsstöd och kan deprekeras i en senare DSL-version.

### 6.6 `reason`

Fri sträng på `route`-nivå. Infogas i decision-explanation när regeln matchar.

## 7. Vokabulär

Policy ska använda exakt denna v1-vokabulär. Termerna är konsumerade av
JobDescriptor (ISSUE-014), classifier (ISSUE-017/018) och registry (ISSUE-009/010).

### task_type

`trivial_git`, `simple_shell`, `summarization`, `simple_code_edit`,
`hard_code_debugging`, `security_review`, `database_migration`,
`long_context_analysis`, `creative_copy`, `unknown_high_risk`.

(`simple_chat` finns i descriptor-vokabulären men styrs i v1 av defaultregeln,
inte av specifika policyregler.)

### risk_level

`low`, `medium`, `high`, `critical`.

### sensitivity

`none`, `source_code`, `legal`, `financial`, `health`, `security_classified`, `pii`, `secrets_possible` — i stigande rankningsordning; rankningen får aldrig ändras (deployade policies matchar mot den).

### router_mode

`auto`, `cheap`, `balanced`, `premium`, `disabled`.

### model_profile (bred)

`cheap`, `balanced`, `premium`.

### model_profile_name (intern registry)

Strängar definierade av registry (ISSUE-010), exempel: `cheap-general`,
`balanced-coder`, `premium-reasoning`. Default-policy bör i första hand
referera till breda profiler; specifika namn används när registry har
publicerat dem.

### capabilities

`streaming`, `tool_use`, `json_schema`, `vision`, `long_context`. Styrkor
används vid behov: `code`, `summarization`, `hard_reasoning`, `security_review`.

### registry_version

Policy får referera till `registry_version` i `metadata` eller i test-/debug-output,
men inte i `when` eller `route` på fast path.

## 8. Prioritet och merge

1. **Block.** Den första regel som matchar med `block` avslutar utvärderingen
   och returnerar ett blockerat beslut.
2. **Force.** Matchade `force`-fält sätter icke-sänkbara krav. För ett givet
   force-fält gäller **first-match-wins**: senare regler som försöker sätta
   samma fält ignoreras, och en explanation läggs till om att senare regel
   skuggats av tidigare.
3. **Constraints.** Ackumuleras över alla matchande regler enligt fält-specifika
   merge-regler (snitt/union/min, se §6.3).
4. **Defaults.** Appliceras sist och får inte bryta block, force eller
   constraints. Sätter värden för fält som ingen tidigare regel rört.
5. **Hints/route-hints.** Mappas vid kompileringstid till motsvarande
   force/constraints/defaults och följer samma prioritet.
6. **User/request override.** Klient-metadata och request-hints får inte bryta
   säkerhets- eller tenantpolicy. Override som strider mot `force` eller
   `constraints` resulterar i `force`/`constraints` som vinner och en
   explanation som rapporterar konflikten.

Implementation ska returnera explanation-fragment för varje regel som matchat,
för varje force-fält som skuggats, och för varje constraint som tvingat fram
filtrering.

## 9. Komplett default-policy

```yaml
version: pv_2026_05_19
metadata:
  owner: platform
  description: Default Policy DSL v1 for Sluss routing
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: block_disabled_router
    when:
      router_mode: disabled
    route:
      block:
        code: router_disabled
        reason: Router mode disabled by policy

  - id: trivial_git_uses_cheap
    when:
      task_type: trivial_git
      risk_level: low
    route:
      defaults:
        model_profile: cheap
        max_cost_usd: 0.002

  - id: simple_code_uses_balanced_coder
    when:
      task_type: simple_code_edit
      risk_level:
        in: [low, medium]
    route:
      defaults:
        model_profile_name: balanced-coder
      constraints:
        require_capabilities: [tool_use]

  - id: auth_payments_security_are_premium
    when:
      any_file_matches:
        - "**/auth/**"
        - "**/*auth*"
        - "**/payments/**"
        - "**/*payment*"
        - "**/security/**"
    route:
      force:
        model_profile_name: premium-reasoning
        verifier: true
      constraints:
        require_capabilities: [tool_use, json_schema]

  - id: migrations_are_premium
    when:
      task_type: database_migration
    route:
      force:
        model_profile: premium
        verifier: true
      constraints:
        require_capabilities: [long_context]

  - id: long_context_requires_capability
    when:
      task_type: long_context_analysis
    route:
      constraints:
        require_capabilities: [long_context]
      defaults:
        model_profile: premium

  - id: unknown_high_risk_is_premium
    when:
      task_type: unknown_high_risk
      risk_level:
        in: [high, critical]
    route:
      force:
        model_profile: premium
        verifier: true

  - id: default_balanced
    when: {}
    route:
      defaults:
        model_profile: balanced
```

## 10. Policytest

Varje policy ska ha testfall som senare körs av policy test runner (ISSUE-024):

```yaml
case: auth file should be premium
input:
  task_type: simple_code_edit
  risk_level: medium
  files_touched:
    - src/auth/session.ts
expected:
  model_profile_name: premium-reasoning
  verifier: true
  require_capabilities: [tool_use, json_schema]
  matched_rules:
    - auth_payments_security_are_premium
```

Testformatet använder samma v1-vokabulär. `expected` får uttryckas mot `model_profile`
eller `model_profile_name` beroende på vilken nivå policyn arbetar på.

## 11. Automatisk policyrekommendation

Routern kan föreslå per-taskklass-ändringar av tier-golvet utifrån den uppmätta
cost/quality-frontieren (Fas 4: *"lära routing från outcome-data"*). Detta är
**rådgivande och aldrig auto-applicerat** — en operatör granskar och redigerar
policy för hand (governance).

```bash
make eval-recommend          # eval-rapport + rekommendationer
# eller direkt, med trösklar:
go run ./cmd/eval-report -recommend -target-pass 0.9 -min-samples 5
```

För varje taskklass jämförs routerns **baslinjegolv** (`engine.BaselineTier`) med
den **billigaste modell som är tillräckligt bra** (uppmätt eval pass rate ≥
`-target-pass`, med minst `-min-samples` mätningar):

| Rekommendation | Betydelse |
|---|---|
| `downgrade` | En billigare tier klarar evals — sänk golvet, spara kostnad |
| `upgrade` | Ingen modell på/under golvet når målet — höj golvet |
| `keep` | Billigaste tillräckligt bra modell ligger redan på golvet |
| `no_model_meets_target` | Ingen modell nådde målet — utöka evaltäckning |
| `insufficient_data` | För få eval-samples för att lita på pass rate |

Artefakter skrivs till `eval-report/`:

- `policy-recommendations.txt` — läsbar tabell med skäl och kostnadsdelta.
- `policy-recommendations.yaml` — ett granskningsbart policy-snippet (`when.task_type`
  + `route.force.model_profile`) som validerar mot policy-parsern; klistra in de
  regler du håller med om i din policyfil.

Rekommendationerna antar taskklassens nominella (låga) risk; policy kan fortfarande
eskalera per request. Detta kompletterar eval-driven scoring (som påverkar
*scoring*) med ett förslag på *golv/force*.

## 12. Status

Detta issue (ISSUE-020) är docs-only. Ingen produktkod, inga fixtures, inga
migrationer och inga `.ai`-strukturer har skapats som del av själva DSL-specen.
Parser/validator/compiled-cache/explanations/test-runner ligger i ISSUE-021–024.
Eftersom inget Go-kod producerats krävs inga `go test`-körningar för
verifiering; verifiering sker via markdown-review och konsistenskontroll mot
`01-architecture/04-routing-engine.md`, `01-architecture/06-model-registry.md`,
`01-architecture/07-policy-engine.md` och `06-engineering/02-job-descriptor-schema.md`.
