# MVP-checklista

## Funktion

- [ ] `/v1/chat/completions` fungerar.
- [ ] `model: auto` fungerar.
- [ ] Minst tre modellprofiler finns.
- [ ] Policy kan tvinga cheap/balanced/premium.
- [ ] Fallback fungerar.
- [ ] Streaming fungerar tillräckligt för beta.
- [ ] `/router/decision` dry-run finns.

## Latency

- [ ] Feature extraction p95 < 20 ms.
- [ ] Policy eval p95 < 10 ms.
- [ ] Total router overhead p95 < 100 ms.

## Observability

- [ ] Decision log.
- [ ] Attempt log.
- [ ] Cost metrics.
- [ ] Latency metrics.
- [ ] Provider health.

## Säkerhet

- [ ] API keys hashas.
- [ ] Provider secrets lagras säkert.
- [ ] Prompt logging kan stängas av.
- [ ] Policy kan blockera provider.

## Evals

- [ ] Minst 50 evalfall.
- [ ] Policy tests.
- [ ] Eval smoke i CI.
