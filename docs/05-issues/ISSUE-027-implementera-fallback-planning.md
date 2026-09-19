# ISSUE-027: Implementera fallback planning

## Labels
- `epic: EPIC-05`
- `priority: P0`
- `type: backend`
- `sprint: 05`
- `category: enhancement`
- `state: done`

## Intent

Implementera fallback planning som bygger fallbackkedjan innan första provider call. Kedjan ska vara deterministic, policy-safe och kompatibel med streamingregeln: fallback är fri före första token men begränsad efter att data skickats till klient.

## Implementation Contract

- Input ska vara selected primary candidate, filtered/scored candidate list, `JobDescriptor`, policy constraints och provider health snapshot.
- Output ska ingå i `RouteDecision`: primary route, fallback attempts i ordning, timeout/retry metadata och explanation fragments.
- Fallbackkedjan ska skapas före provider execution. Den får inte byggas först efter timeout, eftersom det bryter latency- och recoverydesignen.
- Policy constraints gäller även fallback: denied provider/model får inte dyka upp i fallbackkedjan; forced premium/verifier/securitykrav får inte sänkas.
- Riskabla uppgifter ska fallbacka uppåt eller sidledes, inte nedåt. Exempel: `security_review`, `database_migration`, `unknown_high_risk` och auth/payment prompts ska inte falla tillbaka till cheap model.
- Low-risk cheap routes kan fallbacka till balanced om cheap provider är nere/rate-limited, men budget caps och policy maxkostnad måste respekteras.
- Undvik duplicerade providers/models i kedjan när det inte ger ny redundans. Preferera provider diversity för samma modellklass om registry/health stödjer det.
- Streaming: fallback efter första token är out of scope i plannern utöver att beslutet ska markera `fallback_allowed_before_first_token=true` och lämna restartpolicy till streaming execution issue.
- Timeout/retry relation ska vara tydlig: plannern anger attempt order och per-attempt timeout från policy/defaults; executor ansvarar för att faktiskt retrya.
- Explanations ska visa varför fallback valdes eller uteslöts utan prompttext.

## Files / Packages

- Förväntad produktkod: `internal/router` route decision/fallback planner.
- Förväntad integration: candidate filtering från ISSUE-025 och scoring från ISSUE-026.
- Förväntade tester: table-driven tests med fake candidates, policy constraints och health snapshots.
- Håll ändringen till fallback planning och route-decision metadata. Provider execution retry hör till ISSUE-029/030.

## Acceptance Criteria

- Route decision innehåller primary och deterministisk fallbackkedja.
- Fallbackkedjan respekterar policy allow/deny, forced profile och required capabilities.
- High-risk/security/migration/auth/payment tasks fallbackar inte till underpowered cheap models.
- Cheap/low-risk task kan fallbacka till balanced när policy och budget tillåter.
- Provider health kan påverka fallbackordning eller exkludera hard-down fallback.
- Streamingbeslut markerar fallbackgränsen före first token.
- Explanations finns för selected fallback och excluded fallback candidates.

## Tests / Verification

- Unit test: primary premium on provider A får fallback premium/provider B när task är high risk.
- Unit test: denied provider syns varken som primary eller fallback.
- Unit test: cheap low-risk route får balanced fallback när cheap provider är unhealthy.
- Unit test: high-risk auth/security route får inte cheap fallback.
- Unit test: duplicate candidate/provider undviks enligt kontraktet.
- Unit test: no valid fallback ger tom kedja med explicit reason, inte error om primary är giltig.
- Unit test: streaming route markerar fallback-before-first-token boundary.
- Kör fokuserade Go-tester för router/fallbackpaketet.

## Out of Scope

- Ingen faktisk retry/execution-loop.
- Ingen fallback efter första streamade token.
- Ingen provider health worker.
- Ingen scoreformel.

## Dependencies

- ISSUE-025 för filtered candidates.
- ISSUE-026 för scored/selected primary candidate.
- ISSUE-029 för provider timeout/retry execution.
- ISSUE-030 för fallback före first token i streaming execution.
- `01-architecture/04-routing-engine.md`
- `01-architecture/08-provider-abstraction.md`

## Subagent Notes

- Bygg kedjan tidigt och logga reasoning. Det gör provider failures mycket lättare att debugga senare.
- Var konservativ med risk: fallback får rädda availability men inte bryta safety eller tenantpolicy.
- Håll planner ren från provider calls så den förblir snabb och testbar.

## Klar när

- Fallback planner producerar policy-safe route-decision fallbackkedja före provider execution.
- Streaming boundary och no-fallback cases är explicita.
- Acceptance criteria och fokuserade tester passerar.
