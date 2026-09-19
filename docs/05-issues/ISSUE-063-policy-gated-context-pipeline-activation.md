> *Created during codebase review follow-up.*

# ISSUE-063: Policy-gated context pipeline activation

## Labels
- `epic: EPIC-04`
- `priority: P1`
- `type: backend`
- `state: done`
- `adr: ADR-0013`
- `sprint: 04`
- `category: enhancement`

## Mål

Replace the global-only `ROUTER_CONTEXT_PIPELINE_ENABLED` activation path with server-controlled tenant/project policy gates before real context processors are introduced.

## Bakgrund

ADR-0013 says context processors mutate customer input and must be opt-in via tenant policy, not request headers. ISSUE-062 intentionally shipped only the interface, no-op pipeline, and global feature flag. That is fine for the skeleton, but before RTK/dedup/redaction/summarization processors land, the runtime must decide activation from compiled policy context.

## Acceptanskriterier

- Policy DSL can express whether the context pipeline is enabled for a tenant/project and, if needed, task class.
- The chat request path checks server-side policy context before running processors.
- The global env flag is retained only as an operator kill switch or rollout gate, not as the only activation control.
- Client headers or request metadata cannot enable context processors.
- Tests prove disabled-by-default behavior, policy-enabled behavior, and client metadata/header bypass rejection.
- Fast path remains deterministic and within the context processor latency budget from ADR-0013.

## Tekniska noter

- Build on `internal/contextproc.Pipeline` from ISSUE-062.
- Prefer integrating with the policy parser/cache work from ISSUE-020 through ISSUE-022.
- Keep streaming skipped until the streaming-specific design lands.
- Log applied/skipped processors without logging raw prompt content.

## Klar när

- Activation is policy-controlled and covered by focused tests.
- `go test ./...`, `go vet ./...`, and `make lint` pass.
- `.ai/tasks.json` and `05-issues/issue-index.md` reflect the task state.

## Implementation (klar 2026-06-12)

- Policy DSL stöder `route.force.context_pipeline: true|false`; default är off och kompilerad policy avgör aktivering server-side.
- Runtime bootstrap i `cmd/router` wirar `PolicyCache` och kan ladda policyfil via `ROUTER_POLICY_PATH`; `ROUTER_CONTEXT_PIPELINE_ENABLED` fungerar som operator rollout/kill switch, inte som enda aktiveringskälla.
- Chat-path kör context pipeline efter policy/routing-beslut och precis före non-streaming provider-call; streaming skippar fortsatt pipelinen.
- Headers och client metadata kan inte slå på processors; tester täcker default-off, policy-enabled, kill switch och bypass-försök.
