# Verifier och escalation

> **Status (superseded i delar):** Den skeppade realiseringen av
> "premium + verifier" är **council mode** (`internal/council`, se
> `docs/council.md`) — en deterministisk `ConsensusJudge` över flera
> modellsvar, ingen LLM-verifier och ingen human-in-the-loop i request-pathen.
> Cascade-resonemanget nedan beskriver designrymden, inte implementationen.

## Syfte

Verifiering och eskalering minskar risken när billigare modell används.

## Cascade

```mermaid
flowchart TD
    Req[Request] --> Cheap[Cheap/Balanced attempt]
    Cheap --> Check{Verifier / objective check}
    Check -->|Pass| Return[Return response]
    Check -->|Fail| Premium[Escalate to premium]
    Premium --> Return
```

## När verifier behövs

- Security review.
- Database migration.
- Auth/payment-kod.
- Produktion nämns.
- Hög diff-risk.
- Låg classifier confidence.
- Användaren begär `requires_verifier`.

## Verifiermetoder

### Objektiva checks

- Tests pass/fail.
- Typecheck.
- Lint.
- JSON schema validation.
- SQL dry-run.

### LLM verifier

- Kontrollera att svaret följer instruktion.
- Kontrollera risker.
- Kontrollera hallucinerade APIs.

### Human verifier

Enterprise eller high-risk workflows kan kräva approval.

## Escalation triggers

- Verifier failed.
- Provider timeout.
- Low confidence.
- User feedback rejected.
- Policy requires premium after failed cheap attempt.

## Kostnadskontroll

Verifiering kan bli dyrare än att välja premium direkt. För high-risk tasks kan route direkt till premium vara billigare totalekonomiskt.

## Beslutsregel

Använd cascade främst när:

- Tasken ofta är enkel men ibland svår.
- Billig modell har hög pass rate.
- Verifieringen är billig eller objektiv.

Routea direkt till premium när:

- Fel är dyrt.
- Verifier är dyrt.
- Tasken är komplex från start.
