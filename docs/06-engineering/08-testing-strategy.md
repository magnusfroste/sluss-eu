# Testing strategy

## Testnivåer

### Unit tests

- Feature extraction.
- Policy matching.
- Model scoring.
- Cost estimation.
- Provider adapter mapping.
- Error normalization.

### Integration tests

- OpenAI-compatible endpoint.
- Streaming.
- Fallback.
- Auth.
- Logging.
- Budget limit.

### Contract tests

- Provider adapters.
- Response formats.
- Tool calls.
- JSON schema.

### E2E tests

- Client sends prompt.
- Router selects model.
- Provider mock returns response.
- Event log created.
- Dashboard aggregate updates.

### Eval tests

- Golden prompt dataset.
- Expected route.
- Cost/quality comparison.

## Mock provider

Bygg en mock provider som kan simulera:

- Normal response.
- Streaming.
- Timeout.
- Rate limit.
- 5xx.
- Invalid response.
- Token usage.

## Regressionstest

Varje incident med felrouting ska bli ett regressionstest.

## Performance tests

- 100 RPS non-streaming.
- 1 000 samtidiga streaming connections, om relevant.
- Policy reload under trafik.
- Event queue backlog.

## Testdata

Prompttext i testdata ska inte innehålla verkliga secrets eller kunddata.
