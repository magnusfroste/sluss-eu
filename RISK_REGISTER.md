# Riskregister

| Risk | Sannolikhet | Impact | Mitigation |
|---|---:|---:|---|
| Router väljer för svag modell för high-risk task | Medium | Hög | Konservativ policy, verifier, regressionstester |
| Routing overhead blir för hög | Medium | Hög | Fast path utan LLM/DB, latencybudget, tracing |
| Provider API-skillnader skapar buggar | Hög | Medium | Adapter contract tests, mock provider |
| Kostnadsbesparing uteblir | Medium | Medium | Evals, spend simulator, taskklassanalys |
| Promptar innehåller secrets | Hög | Hög | Secret masking, prompt logging off, provider allowlist |
| Policy blir svår att förstå | Medium | Medium | Explanations, policy tests, templates |
| Dashboard visar fel kostnad | Medium | Medium | Usage reconciliation, aggregation tests |
| Eventlogg växer för snabbt | Medium | Medium | Retention, sampling, ClickHouse senare |
| Fallback fungerar inte med streaming | Medium | Hög | Fallback före first token, tydliga regler |
| Modeller ändrar pris/kapabilitet | Hög | Medium | Registry versionering, manuell review |
