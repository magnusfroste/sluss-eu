# Säkerhet och privacy

## Hotmodell

Primära risker:

- Promptar innehåller secrets.
- Känslig kod skickas till otillåten provider.
- API keys läcker.
- Logs lagrar mer data än nödvändigt.
- Fel policy leder till compliance-brott.
- Provider credentials missbrukas.

## Säkerhetsprinciper

- Minsta möjliga lagring av promptinnehåll.
- API keys hashas, inte lagras i klartext.
- Provider secrets ligger i secrets manager.
- Policy kan blockera providers per projekt.
- Beslut loggas med policyversion.
- Secrets maskas före loggning och före provideranrop om policy kräver.

## Secret masking

Maska vanliga mönster:

- API keys.
- JWT.
- Private keys.
- Database URLs.
- Access tokens.
- `.env`-värden.

Maskning ska loggas som event men inte spara originalvärdet.

## Data retention

Rekommendation:

| Datatyp | MVP retention |
|---|---:|
| Request metadata | 90 dagar |
| Prompttext | Av som default eller 7 dagar |
| Decision logs | 180 dagar |
| Cost aggregates | 1 år |
| Audit events | 1 år |

## Provider policy

Varje tenant/projekt ska kunna ha:

- Allowed providers.
- Denied providers.
- Allowed regions, om provider stöder det.
- Data retention preference.
- Prompt logging on/off.

## Audit events

Logga:

- API key skapad/roterad/revoked.
- Policy aktiverad.
- Provider aktiverad/inaktiverad.
- Blockerad request.
- Budgetlimit nådd.
- Manuell override.

## Auth

MVP:

- Bearer API keys.
- Hashade keys.
- Key scopes.

Beta:

- RBAC.
- SSO.
- Project-level keys.
- Key rotation reminders.

## Säkerhetsacceptans för beta

- Alla API keys hashas.
- Provider keys ligger inte i vanlig DB.
- Promptlogging kan stängas av per tenant.
- Policy kan blockera externa providers.
- Minst grundläggande secret scanning finns.
- Audit log finns för policyändringar.
