# Caching

## Cachetyper

### API key cache

- Key hash -> tenant/project/scopes.
- TTL kort, exempel 60 sekunder.
- Invalidation vid revoke.

### Policy cache

- Tenant/project -> compiled policy.
- Versionerad.
- Hot reload.

### Model registry cache

- Global och tenantfiltrerad registry.
- Uppdateras vid adminändring.

### Provider health cache

- Uppdateras av health worker.
- Läses vid routing.

### Decision cache

- Normaliserad prompt + metadata + policyversion -> decision.
- Kort TTL.
- Endast låg-risk eller deterministic cases.

## Cache key

```text
decision_cache_key = hash(
  normalized_prompt_fingerprint,
  metadata_fingerprint,
  policy_version,
  registry_version,
  tenant_id
)
```

## När decision cache inte ska användas

- High-risk tasks.
- Prompt innehåller känslig data.
- Provider health är degraded.
- Budgetstatus har ändrats.
- Request kräver verifier.

## Risk

Cache får inte göra att gammal policy används. All cache ska vara policyversionerad.
