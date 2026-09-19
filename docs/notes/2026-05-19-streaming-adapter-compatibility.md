# 2026-05-19 - Streaming Adapter Compatibility

## Context

ISSUE-012 added streaming support after the non-streaming provider adapter already existed. Existing mock and fake adapters only implemented `Complete`, while the streaming path needed provider-owned chunk delivery.

## What I Learned

Adding streaming directly to the base provider interface would force every existing adapter and test double to implement streaming even when it cannot support it. A small optional `StreamingAdapter` interface lets non-streaming adapters stay compatible and makes unsupported streaming fail before the first chunk with a normal provider error.

First-token fallback also needs per-attempt cancellation. If the handler abandons a provider stream after a setup error or first-token timeout without canceling that attempt's context or draining its chunk channel, adapter goroutines can remain blocked trying to send the late first chunk.

## Reuse Rules

- Keep `Adapter.Complete` as the non-streaming minimum provider contract.
- Gate streaming with an optional `StreamingAdapter` check and fail before headers/body are written when unsupported.
- Treat the first emitted stream chunk as the fallback boundary; after that, report interruption in-stream and do not retry.
- Give each streaming candidate its own cancelable context and cancel it whenever fallback moves to the next candidate before a chunk is flushed.
- Keep provider SSE parsing minimal at the adapter boundary and let the HTTP handler own client-facing SSE framing.

## Failure Signals

- Adding a streaming method to `Adapter` breaks mock adapters or non-streaming fakes.
- Unsupported streaming writes `text/event-stream` before returning a JSON setup error.
- Mid-stream provider errors return HTTP JSON after chunks have already been flushed.
- A first-token timeout continues to the next candidate while the abandoned adapter goroutine still owns an open response body or an unconsumed chunk channel.
- Tests only cover happy-path `[DONE]` and miss interruption after the first chunk.

## Next Checklist

- When adding a new provider adapter, implement `StreamingAdapter` only if the provider supports streaming.
- Keep setup errors before first chunk mapped through `mapProviderError`.
- Add one test for unsupported streaming and one for interruption after first chunk for every new streaming seam.
- Add timeout/fallback tests that prove the abandoned candidate receives cancellation before the next candidate starts.
