# Threat model

## Assets

- Kundpromptar.
- Källkod.
- API keys.
- Provider credentials.
- Routingpolicy.
- Kostnadsdata.
- Audit logs.

## Actors

- Legitima användare.
- Illvillig användare med API key.
- Extern angripare.
- Felkonfigurerad provider.
- Intern admin med för bred access.

## Threats

### Prompt data leakage

Risk att känslig data skickas till otillåten provider.

Mitigation:

- Provider allowlist.
- Sensitive project policy.
- Secret masking.
- Prompt logging off by default.

### API key compromise

Risk att en nyckel används för att skapa kostnader eller exfiltrera data.

Mitigation:

- Scopes.
- Budgets.
- Rate limits.
- Key rotation.
- Anomaly detection.

### Policy bypass

Risk att klient sätter `model` explicit för att gå runt policy.

Mitigation:

- Policy övertrumfar override.
- Logga overrideförsök.
- Blockera otillåtna modeller.

### Provider credential leak

Mitigation:

- Secrets manager.
- Aldrig logga provider headers.
- Separata credentials per environment.

### Cost attack

Mitigation:

- Max tokens.
- Max cost per request.
- Rate limits.
- Budget caps.

## Säkerhetstester

- Secret masking tests.
- API key hashing tests.
- Policy bypass tests.
- Rate limit tests.
- Prompt logging disabled tests.
