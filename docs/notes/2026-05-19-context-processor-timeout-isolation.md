# 2026-05-19 - Context Processor Timeout Isolation

## Context

ISSUE-062 added a fail-open context-processor pipeline where processors receive a mutable request and may time out. The first implementation passed the live request pointer into the processor goroutine and returned on hard timeout. A follow-up review found the same class of issue can survive a top-level clone if nested JSON-ish fields remain shared.

## What I Learned

A hard timeout cannot stop a Go goroutine. If a timed-out processor still holds shared mutable request state, it can mutate the request after the pipeline has already continued to the provider adapter. The race-enabled test gate caught the original live-pointer case as a data race, and the product risk is worse: late context mutation could affect an in-flight provider call.

For `NormalizedModelRequest`, cloning must include pointer fields like `Temperature` and `MaxTokens`, plus nested `map[string]any` and `[]any` values inside `Tools` and `Metadata`.

## Reuse Rules

- Run context processors against a deeply cloned candidate request.
- Commit candidate mutations back to the live request only when the processor finishes successfully before timeout.
- Treat hard timeout as "stop waiting", not as goroutine cancellation.
- Keep fail-open paths isolated from shared mutable request state.
- Add clone regression tests for nested JSON maps/slices before adding processors that mutate `Tools` or `Metadata`.

## Failure Signals

- `go test -race` reports request mutation after a timeout.
- A processor timeout test passes without `-race` but fails with `-race`.
- Provider requests show unexpected late mutations after a skipped processor.
- A clone test can mutate nested `Tools` or `Metadata` in the clone and observe the original changing too.

## Next Checklist

- [x] Confirm processors receive deeply cloned request state before adding real processors.
- [ ] Add race-enabled tests for processors that may ignore context cancellation.
- [ ] Keep per-processor timeout behavior fail-open and mutation-isolated.
