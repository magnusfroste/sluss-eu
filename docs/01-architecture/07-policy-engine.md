# Policy engine

> **DSL-kontrakt:** Schemat för policy (top-level-fält, `when`, `route`, prioritet,
> vokabulär och default-policy) definieras av Policy DSL v1 i
> [`06-engineering/01-routing-policy-reference.md`](../06-engineering/01-routing-policy-reference.md).
> Detta dokument beskriver engine-arkitekturen; exemplen nedan använder
> `tier`-hints som DSL v1 mappar till `force`/`constraints`/`defaults` med
> `model_profile`-vokabulär.
>
> **Lagring idag:** YAML-formen används av de inbyggda compliance-packen
> (`builtin:pii-local`, `builtin:nis2-baseline`, `builtin:dora-baseline`,
> `builtin:gdpr-sovereign`). Live-redigering sker som no-YAML-regler i
> policy-konsolen, lagrade i SQLite och hot-reloadade — YAML-filer på disk är
> inte den operativa redigeringsvägen.

## Syfte

Policy engine avgör vilka rutter som är tillåtna eller tvingade. Den ska vara snabb, förklarbar och versionerad.

## Policytyper

- Allow/deny providers.
- Allow/deny modeller.
- Tvinga modellnivå för vissa tasks.
- Tvinga verifiering.
- Sätta budgettak.
- Blockera känslig data.
- Sätta retention.
- Sätta fallbackkedja.

## Exempelpolicy

```yaml
version: pv_2026_05_19
rules:
  - id: trivial_git_is_cheap
    when:
      task_type: trivial_git
      risk_level: low
    route:
      tier: cheap
      verifier: false
      max_cost_usd: 0.002

  - id: auth_requires_premium
    when:
      any_file_matches:
        - "**/auth/**"
        - "**/*auth*"
        - "**/middleware.*"
    route:
      tier: premium
      verifier: true
      reason: "Auth-related files require premium reasoning"

  - id: database_migration_requires_verifier
    when:
      any_file_matches:
        - "**/migrations/**"
        - "**/*.sql"
    route:
      tier: premium
      verifier: true
      fallback_tier: premium

  - id: block_external_for_sensitive_project
    when:
      project: internal-security
    constraints:
      allowed_providers:
        - self_hosted
```

## Policyutvärdering

1. Läs tenantpolicy.
2. Läs projectpolicy.
3. Läs request-level overrides.
4. Applicera blockregler först.
5. Applicera tvingande regler.
6. Applicera constraints.
7. Skicka kvarvarande kandidatset till routing engine.

## Prioritet

Ordning:

1. Block.
2. Compliance/säkerhet.
3. Tenant budget.
4. Project policy.
5. User override.
6. Router scoring.

User override får inte bryta säkerhetspolicy.

## Beslutsförklaring

Policy engine ska returnera explanation fragments:

```json
[
  "Rule auth_requires_premium matched file src/auth/session.ts",
  "Verifier required by project policy",
  "Provider external_x denied by tenant policy"
]
```

## Kompilering

För låg latency bör policy kompileras till:

- Prefix-/globmatchers.
- Hashsets för providers och modeller.
- Prioriterade regelgrupper.
- Predikatfunktioner.

## Testning

Varje policy ska kunna testas utan provideranrop:

```bash
make test-policy                 # golden cases i internal/policy
curl -X POST :8080/router/policy/dryrun -d '{"prompt":"..."}'   # konsolens dry-run
```

Varje testfall bör innehålla:

- Input descriptor.
- Förväntad tier.
- Förväntad block/allow.
- Förväntade explanations.
