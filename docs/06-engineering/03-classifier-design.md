# Classifier design

## Mål

Klassificera prompten snabbt nog för fast path-routing.

## V1: regler och features

Feature extractor ska producera signaler utan LLM-call.

### Features

- Antal tecken.
- Uppskattade tokens.
- Kodblock finns/inte finns.
- Filnamn eller pathar.
- Nyckelord.
- Explicit metadata.
- Begärda tools.
- Outputformat.
- Förekomst av stack traces.
- Förekomst av SQL, migrations eller auth/payment-termer.

### Taskklassregler

Exempel:

```text
contains "commit message" + diff => trivial_git
contains stack trace + code => hard_code_debugging
contains "migration" or *.sql => database_migration
contains "security" or "vulnerability" => security_review
short prompt + no code + no tools => simple_chat
```

## Deterministisk by design (beslut)

Klassificeraren förblir regelbaserad — **ingen ML/LLM i beslutsvägen**. Det är
en produktegenskap, inte en teknisk begränsning: deterministisk klassificering
är förklarbar och reviderbar, vilket är kärnan i control + evidence-positionen
(en CISO ska kunna försvara varje beslut för en auditor).

Det finns ett *offline* klassificerarexperiment (ISSUE-058,
`internal/classifier/experiment.go` + `docs/fixtures/`) som jämför en tränad
modell mot reglerna på ett dataset — som forskningsverktyg för att förbättra
reglerna, aldrig som serving-path.

## Confidence

Classifier ska returnera confidence.

```json
{
  "task_type": "hard_code_debugging",
  "confidence": 0.82,
  "risk_level": "high",
  "signals": ["stack_trace", "auth_keyword", "production_keyword"]
}
```

Om confidence är låg och riskindikatorer finns, välj konservativ route.

## Testning

- Unit tests för regler.
- Golden dataset.
- Regressionstest mot tidigare felrouting.
- Latencytest för feature extractor.

## Anti-pattern

Använd inte en premium LLM för att klassificera varje prompt. Det dödar produktens latency- och kostnadsargument.
