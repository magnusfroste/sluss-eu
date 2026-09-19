# ISSUE-018: Implementera riskklassregler

## Labels
- `epic: EPIC-03`
- `priority: P0`
- `type: backend`
- `sprint: 03`
- `category: enhancement`
- `state: done`

## Intent

Implementera deterministiska riskklassregler som sätter `risk_level` och sensitivity-hints för router/policy baserat på feature-output, taskklass och tenant/policy-kontext.

## Implementation Contract

- Riskvokabulär ska vara exakt `low`, `medium`, `high`, `critical`.
- Riskregler ska använda feature-output från ISSUE-016 och taskklass/signals från ISSUE-017.
- Auth, payments, migrations och security ska höja risk minst till `high` när de kombineras med kod, path, produktion eller ändringsintention.
- Production/urgent language ska höja risk: `production`, `prod`, `incident`, `urgent`, `ASAP`, `down`, `outage`, `hotfix`, `rollback`.
- Secrets/PII hints ska höja risk och sensitivity: `api_key`, `secret`, `token`, `password`, `ssn`, `personnummer`, `email`, `phone`, `address`, `customer data`.
- `security_review`, `database_migration` och `unknown_high_risk` ska aldrig klassas `low`.
- `trivial_git`, `simple_chat` och `summarization` kan vara `low` endast när risk-/sensitivity-signaler saknas.
- Riskregler ska producera signals/reasons som policy senare kan använda.
- Policy-escalatable risk: klientmetadata får aldrig sänka risk under härledda signaler; policy ska kunna eskalera risk efter classifiern utan att ändra promptanalysis.
- Inga nätverk, DB eller LLM-anrop i riskklassningen.

## Files / Packages

- Förväntad produktkod: intern classifier/policy-adjacent package, till exempel `internal/classifier` eller `internal/router/risk`.
- Förväntad integration: `JobDescriptor.risk_level` och `sensitivity` fylls innan routing score/policy evaluation.
- Förväntade tester: table-driven tests för risknivå, sensitivity och policy-escalation boundaries.
- Håll produktändringen till riskregler, descriptor-integration och fokuserade tester.

## Acceptance Criteria

- Auth-relaterad kod/path höjer risk till minst `high`.
- Payment/billing/checkout/Stripe-signaler höjer risk till minst `high`.
- SQL migration/schemaändring höjer risk till minst `high`.
- Security/vulnerability/secret-signaler höjer risk till minst `high`, och till `critical` om produktion/incident eller exploit-semantik också finns.
- Production/urgent language höjer risk minst ett steg när det kombineras med kod, infra, auth, payments, migrations eller security.
- PII/secrets hints sätter relevant `sensitivity` (`pii`, `secrets_possible` eller motsvarande från schema) och höjer risk.
- Klientmetadata som påstår låg risk kan inte sänka risk när extractor-regler hittat högre risk.
- Risk reasons/signals är loggsäkra och innehåller inte rå prompttext.

## Tests / Verification

- Unit test: `src/auth/session.ts` + kodändring -> minst `high`.
- Unit test: Stripe checkout/payment prompt -> minst `high`.
- Unit test: SQL migration/rollback -> minst `high`.
- Unit test: XSS/security review -> minst `high`.
- Unit test: production outage + secret/exploit -> `critical` om kontraktet stöder critical escalation.
- Unit test: PII hint sätter sensitivity och höjer risk.
- Unit test: klientmetadata `risk_level=low` för auth/payment prompt sänker inte risk.
- Kör fokuserade Go-tester för risk/classifierpaketet.

## Out of Scope

- Ingen fullständig tenant policy engine.
- Ingen provider/model selection.
- Ingen complianceklassificering utöver sensitivity-hints.
- Ingen rå promptlogging.

## Dependencies

- ISSUE-014 för descriptorfält.
- ISSUE-016 för keyword/path/tool/sensitivity-signaler.
- ISSUE-017 för taskklass och classifier signals.
- `06-engineering/02-job-descriptor-schema.md`
- `06-engineering/03-classifier-design.md`
- `01-architecture/07-policy-engine.md`
- `01-architecture/11-security-privacy.md`

## Subagent Notes

- Implementera risk som monoton eskalering: starkare signaler får höja risk, svaga/klientstyrda signaler får inte sänka den.
- Håll sensitivity separat från risk där det går, men låt secrets/PII påverka båda.
- Samma signalnamn som ISSUE-016/017 ska användas i reasons för enkel debugging.

## Klar när

- Riskreglerna sätter `risk_level`, sensitivity och reasons deterministiskt.
- Policy kan eskalera risk utan att klientmetadata kan sänka härledd risk.
- Acceptance criteria och fokuserade tester passerar.
