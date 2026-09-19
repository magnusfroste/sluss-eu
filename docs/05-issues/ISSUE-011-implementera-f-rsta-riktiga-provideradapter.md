# ISSUE-011: Implementera första riktiga provideradapter

## Labels
- `epic: EPIC-06`
- `priority: P0`
- `type: backend`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Implement the first real provider adapter: an OpenAI-compatible HTTP adapter that converts the internal normalized request format into a provider chat completion request and normalizes the provider response, usage, and errors back into internal contracts.

This adapter must be separate from any mock/fake provider used in tests or local development. It must not make routing decisions.

## Implementation Contract

- Add an OpenAI-compatible provider adapter behind the internal provider abstraction.
- Accept a normalized internal request that already contains the selected internal model/registry entry and provider model id.
- Map messages, system content, temperature, max tokens, stream flag, tools, and response format according to the internal normalized request contract.
- Send HTTP requests using configured base URL, API key, timeout, and provider/model metadata.
- Normalize non-streaming responses into the internal response contract.
- Normalize token usage into internal usage fields such as input/prompt tokens, output/completion tokens, total tokens, and any provider raw metadata kept for debugging.
- Normalize provider failures into the internal provider error vocabulary: `provider_timeout`, `provider_rate_limit`, `provider_auth_error`, `provider_5xx`, `provider_bad_request`, `model_unavailable`, and other existing internal error types.
- Keep adapter-level logging/request metadata structured and avoid logging secrets or full prompt bodies by default.
- Do not choose models, apply policy, build fallback chains, or inspect task risk inside the adapter.
- Keep mock/fake adapters available for tests; the real adapter should be registered/configured explicitly.

## Files / Packages

Expected product-code areas for the implementing agent:

- `internal/provider` or equivalent provider interface and normalized request/response contracts.
- `internal/providers/openai` or equivalent OpenAI-compatible adapter package.
- Config surface for provider base URL/API key/timeout/model mapping, if not already present.
- Tests using `httptest.Server` or equivalent local HTTP test server.

Do not change routing policy behavior except to call the adapter through existing provider abstraction seams.

## Acceptance Criteria

- A real OpenAI-compatible HTTP adapter exists separately from the mock provider.
- Adapter maps normalized chat requests to OpenAI-style `/v1/chat/completions` payloads.
- Adapter uses provider model id supplied by registry/profile resolution; it does not derive routing choices itself.
- Successful provider responses normalize content, finish reason, ids/timestamps if available, and usage.
- Provider rate limits, auth failures, bad requests, timeouts, 5xx responses, and unavailable model responses normalize to internal provider errors.
- Adapter can be configured for a compatible provider base URL without code changes.
- Secrets are not logged.
- Existing mock adapter tests keep passing.

## Tests / Verification

- Add unit tests with a local HTTP server for request mapping and headers.
- Add table-driven tests for usage normalization, including missing usage fields.
- Add table-driven tests for status/error mapping: 400, 401/403, 429, 5xx, timeout, and model unavailable body if available.
- Add a test proving the adapter uses the provided provider model id and does not perform profile/routing lookup.
- Run focused provider package tests and any touched router/provider integration tests.

## Out of Scope

- Streaming implementation beyond preserving the `stream` request flag if the interface already supports it. Full streaming skeleton is ISSUE-012.
- Fallback selection or retry policy.
- Provider health scoring.
- Multi-provider abstraction expansion beyond the first OpenAI-compatible adapter.
- Cost estimation.

## Dependencies

- Architecture source: `01-architecture/08-provider-abstraction.md`.
- API shape from `01-architecture/10-api-contracts.md`.
- Registry/profile work that provides selected provider model ids.

## Subagent Notes

- Treat this as a boundary package. The adapter should be boring plumbing, not router intelligence.
- Use standard-library HTTP primitives unless the repo already has an HTTP client wrapper.
- Keep tests hermetic; do not call real OpenAI or other external APIs.

## Klar när

- OpenAI-compatible adapter can send a normalized chat request through a configured HTTP endpoint.
- Response usage and provider errors normalize into internal contracts.
- No routing or policy decisions live in the adapter.
- Provider adapter tests pass without external network calls.

## Closeout 2026-05-19

- Implementerat `provider.OpenAIAdapter` bakom befintlig provider abstraction, separat från `MockAdapter`.
- Adapter använder upstream-valt provider model id i `NormalizedModelRequest.Model`, stödjer konfigurerbar base URL/API key/timeout och normaliserar providerstatus till interna fel.
- Verifierat med hermetiska `httptest`-tester för headers, payload, versioned base URL, usage, timeout och statusmappning.
