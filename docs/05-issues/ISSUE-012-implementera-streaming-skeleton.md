# ISSUE-012: Implementera streaming skeleton

## Labels
- `epic: EPIC-06`
- `priority: P0`
- `type: backend`
- `sprint: 02`
- `category: enhancement`
- `state: done`

## Intent

Add the first streaming skeleton for `POST /v1/chat/completions` when `stream: true`, using SSE-style chunk pass-through from the selected provider adapter while preserving internal observability for first-token timing and stream errors.

This issue establishes streaming shape only. It must not introduce fallback-after-first-token behavior.

## Implementation Contract

- Detect `stream: true` on `/v1/chat/completions` and route through a streaming path after normal policy/registry/provider selection.
- Use the selected provider adapter for streaming chunks; do not bypass the provider abstraction.
- Preserve OpenAI-compatible SSE-style response behavior for clients, including chunk flushing and final stream termination where supported.
- Capture timestamp/latency for first token or first provider chunk sent to the client.
- Record whether `first_token_sent=true` for routing/observability metadata.
- Normalize stream setup errors and mid-stream errors into existing provider error categories, including `stream_interrupted` where appropriate.
- Do not attempt fallback after the first token/chunk has been sent.
- Fallback before first token may remain whatever the existing router supports, but this issue does not need to implement new fallback behavior.
- Keep request-path registry/profile access in-memory and fast-path safe.
- Avoid logging prompt bodies or secrets while still logging request id, provider, selected model/profile, `model_registry_version`, and stream timing/error metadata.

## Files / Packages

Expected product-code areas for the implementing agent:

- HTTP handler for `/v1/chat/completions`.
- Provider abstraction streaming interface or adapter method if not already present.
- OpenAI-compatible adapter streaming support or pass-through shape if ISSUE-011 created the adapter first.
- Router observability/metrics code for first-token timing and stream error state.
- Handler/provider tests using local streams.

Do not create cost estimator logic or new routing policy in this issue.

## Acceptance Criteria

- `POST /v1/chat/completions` with `stream: true` returns an SSE-style streaming response.
- Provider chunks are passed through or normalized consistently with the internal streaming contract.
- The handler flushes chunks progressively instead of buffering the full response.
- First-token/first-chunk timestamp and latency are captured.
- Logs or metrics include `first_token_sent` and relevant request/provider/model identifiers.
- Stream setup errors return a normalized error response before any chunk is sent.
- Mid-stream interruptions are logged/normalized as stream errors without pretending the request succeeded.
- No fallback-after-first-token behavior is implemented.
- Non-streaming chat completions still use the existing non-streaming path.

## Tests / Verification

- Add handler tests for `stream: true` proving chunks are sent in order and the response uses the expected SSE content type/flush behavior.
- Add a test that first-token timing is recorded when the first chunk is sent.
- Add tests for stream setup failure before first chunk.
- Add tests for interrupted stream after at least one chunk, expecting `first_token_sent=true` and stream error logging/metadata.
- Add a regression test that no fallback is attempted after first token.
- Run focused handler/router/provider streaming tests.

## Out of Scope

- Client opt-in restart semantics for fallback after first token.
- Advanced stream transformation, moderation, or buffering.
- Provider health scoring from stream outcomes.
- Full multi-provider streaming parity beyond the first compatible adapter path.
- Cost estimation for streamed output beyond passing usage if the provider supplies it.

## Dependencies

- Architecture source: `01-architecture/08-provider-abstraction.md` streaming guidance.
- API shape from `01-architecture/10-api-contracts.md`.
- ISSUE-011 provider adapter streaming seam, if implemented first.
- Registry/profile selection from EPIC-02 issues.

## Subagent Notes

- Be explicit in code/tests about the first-token boundary. That boundary is the product rule for fallback behavior.
- Use local streaming test doubles; do not call real provider APIs.
- Keep streaming skeleton minimal but observable.

## Klar när

- Streaming chat completions path sends SSE-style chunks through the provider abstraction.
- First-token timing and `first_token_sent` metadata are captured.
- Stream setup and interruption errors are normalized/logged.
- Tests cover chunk pass-through and no fallback after first token.

## Closeout 2026-05-19

- Implementerat optional `provider.StreamingAdapter`, `provider.StreamChunk`, OpenAI-compatible SSE parsing och streaming path i `ChatCompletionsHandler`.
- Streaming svarar med `text/event-stream`, flushar chunks, skickar `[DONE]`, exponerar first-token headers och loggar stream completion/interruption.
- Setup errors returnerar JSON före första chunk; mid-stream errors skrivs som SSE error event utan fallback efter första token.
