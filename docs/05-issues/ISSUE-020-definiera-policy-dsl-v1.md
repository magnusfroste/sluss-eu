# ISSUE-020: Definiera policy DSL v1

## Labels
- `epic: EPIC-04`
- `priority: P0`
- `type: backend`
- `sprint: 04`
- `category: enhancement`
- `state: done`

## Intent

Definiera Policy DSL v1 som det kontrakt som senare parser, validator, compiled-policy-cache och policy engine ska implementera. Detta issue är docs-only: dokumentera YAML/JSON-format, semantik och exempelpolicy utan att skapa Go-produktkod.

DSL:en ska vara deterministisk, snabb att kompilera och kompatibel med fast path-kravet: inga LLM-anrop, externa tjänster eller runtime-beroenden i policyutvärdering före provider call. Policy ska referera till interna modellprofiler och capabilities, inte provider-marknadsnamn.

## Implementation Contract

Dokumentera Policy DSL v1 som ett YAML-format med JSON-ekvivalent struktur. Alla fält ska vara serialiserbara utan YAML-specifika features som anchors, aliases eller custom tags.

Top-level struktur:

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

Definiera dessa top-level fält:

- `version`: obligatorisk policyversion, exempel `pv_2026_05_19`. Ska kunna loggas och ingå i route-decision/debug output.
- `metadata`: valfri dokumentationsyta. V1 ska minst beskriva `owner` och `description` som fria strängar.
- `settings`: obligatoriska defaults och säkerhetsinställningar.
- `rules`: obligatorisk lista av ordnade regler. Regler utvärderas i filordning inom semantikgrupperna nedan.

`settings` ska dokumenteras så här:

- `default_model_profile`: intern profil som används när ingen regel styr hårdare. Tillåtna v1-profiler: `cheap`, `balanced`, `premium`.
- `conservative_unknowns`: när `true` ska okänd eller låg-confidence klassificering aldrig sänka säkerhetsnivå; okänd risk/task får högst routeas till `balanced` eller `premium` enligt defaultpolicy.
- `max_router_overhead_ms`: budget för routerarbete före provider call. V1 default är `100`.
- `default_timeout_ms`: fallback-timeout när varken policyregel eller modellprofil sätter timeout.
- `default_retention`: policy-default för retention, till exempel `standard` eller `none`.

Regelstruktur:

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

Varje regel ska dokumentera:

- `id`: obligatorisk, stabil, unik inom policy, snake_case rekommenderas.
- `description`: valfri mänsklig beskrivning.
- `when`: obligatoriskt matchobjekt. Tomt objekt matchar alltid och används bara för explicit defaultregel.
- `route`: obligatoriskt route-objekt med minst en av `block`, `force`, `constraints`, `defaults` eller kompatibla route hints.

`when` ska stödja följande v1-villkor och tydligt säga att flera fält i samma `when` tolkas som AND:

- `task_type`: sträng eller `{ in: [...] }`.
- `risk_level`: sträng eller `{ in: [...] }`.
- `tenant`: sträng eller `{ in: [...] }`.
- `project`: sträng eller `{ in: [...] }`.
- `prompt_tokens_gt`: heltal.
- `prompt_tokens_lt`: heltal.
- `contains_any`: lista av case-insensitive substrings som får matcha normaliserad prompt/metadata enligt senare implementation.
- `any_file_matches`: lista av globmönster mot JobDescriptorns filsignaler.
- `requires_tool_use`: bool.
- `requires_json_schema`: bool.
- `requires_vision`: bool.
- `sensitivity`: sträng eller `{ in: [...] }`.
- `router_mode`: en av `auto`, `cheap`, `balanced`, `premium`, `disabled`.

Vokabulär som ska användas i exemplen:

- `task_type`: `trivial_git`, `simple_shell`, `summarization`, `simple_code_edit`, `hard_code_debugging`, `security_review`, `database_migration`, `long_context_analysis`, `creative_copy`, `unknown_high_risk`.
- `risk_level`: `low`, `medium`, `high`, `critical`.
- `model_profile`: breda profiler `cheap`, `balanced`, `premium`; specifika interna profilnamn kan vara `cheap-general`, `balanced-coder`, `premium-reasoning` när registry/ISSUE-010 definierar dem.
- Registry koppling: policy får referera till `registry_version` i test/debug metadata och till interna `model_profile`-namn i route, men inte till provider model ids direkt i defaultpolicy.
- Capabilities: använd registry-termerna `streaming`, `tool_use`, `json_schema`, `vision`, `long_context` och vid behov styrkor som `code`, `summarization`, `hard_reasoning`, `security_review`.

`route` ska dokumentera dessa semantiker:

- `block`: stoppar requesten innan provider call. Kan vara `true` eller objekt med `reason`, `code` och valfri `status`. Blockregler har högst prioritet.
- `force`: tvingar val eller miniminivå som inte får sänkas av senare scoring. Stödda v1-fält: `model_profile`, `model_profile_name`, `provider`, `model`, `verifier`, `timeout_ms`, `retention`. `model_profile` använder bred profil, `model_profile_name` använder intern registry-profil som `premium-reasoning`.
- `constraints`: begränsar kandidatset innan scoring. Stödda v1-fält: `allowed_providers`, `denied_providers`, `allowed_models`, `denied_models`, `require_capabilities`, `deny_capabilities`, `max_cost_usd`, `max_latency_ms`, `retention`, `fallback_model_profiles`.
- `defaults`: sätter värden när ingen tidigare block/force/constraint/hint har avgjort dem. Används för policy-defaults och explicit defaultregel.
- Hints på route-nivå får dokumenteras för bakåtkompatibilitet med befintlig referens: `tier`, `model`, `provider`, `fallback_tier`, `fallback_models`, `verifier`, `max_cost_usd`, `timeout_ms`, `retention`, `require_capability`. De ska mappas till `force`, `constraints` eller `defaults` i v1-dokumentationen.

Prioritet och merge-regler:

1. `block` matchar först och avslutar utvärdering med blockerat beslut.
2. `force` matchar därefter och sätter icke-sänkbara krav.
3. `constraints` ackumuleras och begränsar kandidatsetet.
4. Hints/defaults tillämpas sist och får inte bryta block/force/constraints.
5. Regler är ordnade: om två matchade regler sätter samma force-fält ska dokumentationen ange första-match-vinner för säkerhetskritiska fält, och att implementationen ska returnera explanation när senare regel ignoreras.
6. User/request override får inte bryta säkerhets- eller tenantpolicy.

Dokumentera en komplett defaultpolicy i YAML och visa att samma struktur är giltig som JSON. Exempelpolicyn ska minst innehålla:

```yaml
version: pv_2026_05_19
metadata:
  owner: platform
  description: Default Policy DSL v1 for Tokenizer routing
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

## Files / Packages

Expected documentation touch points:

- `06-engineering/01-routing-policy-reference.md`: primary place to document Policy DSL v1 schema, examples, merge semantics and vocabulary.
- `01-architecture/07-policy-engine.md`: update only if needed to point at the v1 reference or align terminology.
- `05-issues/ISSUE-020-definiera-policy-dsl-v1.md`: this issue file.

Expected future implementation packages, for context only:

- `internal/policy`: parser, validator, compiled policy structures and explanations in later issues.
- `internal/router`: consumer of compiled policy outputs.
- `internal/registry`: source of `registry_version`, `model_profile` names and capabilities.
- `internal/classifier` or equivalent: source of `task_type`, `risk_level` and JobDescriptor signals.

Do not create or modify product code in this issue.

## Acceptance Criteria

- Policy DSL v1 is documented as YAML with JSON-equivalent structure.
- The documented schema includes `version`, `metadata`, `settings`, ordered `rules`, `when` and `route`.
- `settings` includes defaults for model profile, conservative unknown handling, router overhead budget, timeout and retention.
- `when` documents v1 match conditions, AND semantics across fields and accepted vocabulary for `task_type`, `risk_level`, `router_mode` and capability-related booleans.
- `route` documents `block`, `force`, `constraints` and `defaults`, including precedence and merge behavior.
- Default policy example includes cheap/balanced/premium routing, premium escalation for auth/payments/security/migrations, long-context capability constraints and a final default rule.
- Vocabulary aligns with planned ISSUE-009 through ISSUE-019 work: model profiles, JobDescriptor fields, task classes, risk levels, registry/model profile naming and capability names.
- The issue remains docs-only and does not require Go code, migrations or runtime wiring.

## Tests / Verification

- No Go tests are required for this docs-only issue.
- Verify with markdown review that the reference doc contains a complete YAML example and that every route semantic has an explicit meaning.
- If a markdown linter is available in the repo, run the narrow docs lint for changed markdown files only.
- Manual consistency check: compare terms against `01-architecture/04-routing-engine.md`, `01-architecture/06-model-registry.md`, `01-architecture/07-policy-engine.md`, `06-engineering/01-routing-policy-reference.md` and `06-engineering/02-job-descriptor-schema.md`.

## Out of Scope

- Implementing parser, validation, compiled policy cache or policy engine behavior.
- Creating registry models, provider adapters or routing/scoring code.
- Defining tenant storage, admin UI or policy distribution.
- Adding `.ai` state, notes, migrations or generated code.
- Supporting YAML anchors, custom YAML tags or provider-specific model ids in default policy.

## Dependencies

- ISSUE-009 and ISSUE-010 define in-memory model registry behavior and internal model profiles.
- ISSUE-014 defines JobDescriptor fields consumed by `when`.
- ISSUE-017 and ISSUE-018 define task and risk classification values.
- ISSUE-021 through ISSUE-024 consume this contract for parser/validation, compiled cache, explanations and policy test runner.
- Architecture references: `01-architecture/07-policy-engine.md` and `06-engineering/01-routing-policy-reference.md`.

## Subagent Notes

- Stay docs-only for ISSUE-020 implementation. Do not create Go files, fixtures, migrations or `.ai` updates.
- Preserve policy-engine latency intent: the DSL can be expressive, but runtime semantics must remain compile-friendly and deterministic.
- Use internal profile names and capability terms, not provider marketing names.
- If terminology conflicts appear between docs, prefer the registry and JobDescriptor vocabulary from ISSUE-009 through ISSUE-019 and call out the alignment in the docs.

## Klar när

- `06-engineering/01-routing-policy-reference.md` has a concrete Policy DSL v1 contract that an implementation agent can follow without guessing schema or precedence.
- The default policy example covers block, force, constraints and defaults.
- The docs say explicitly that this issue produced no product code.
- Verification notes identify that no Go tests were required because this is docs-only.
