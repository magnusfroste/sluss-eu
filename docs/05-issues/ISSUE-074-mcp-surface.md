# ISSUE-074: MCP surface — agents analyze routing data & introspect decisions

## Labels
- `epic: EPIC-11`
- `priority: P2`
- `type: enhancement`
- `sprint: v1.1`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Exponera tokenizers observability + routnings-introspektion som en **MCP-yta** så att (1) en **analys-agent** kan fråga/analysera routnings- och spend-datan konversationellt, och (2) en **dev-agent** (t.ex. Claude Code) kan introspektera routningsbeslut under utveckling. **Read-only i denna iteration.**

## Bakgrund

All data finns redan (`/router/dashboard/data`, request-loggen, `/router/decision`, rostern, provider-health). En MCP-yta gör den **agent-native** (upptäckbar + anropsbar med scheman) istället för att kräva custom-integration. Förstärker observability-USP:n i "agents everywhere". Systerprojektet agentanbud har redan MCP (`mcp_http.py`, `mcp_server.py`) — återanvänd mönstret/transporten.

## Verktyg (read-only, alla wrappar befintlig logik)

- `route_explain(prompt, [messages])` → kör dry-run genom routing-motorn (som `/router/decision`) och returnerar `{task_type, risk, tier, selected_model, provider_model_id, reasons, fallbacks, est_cost}`. **Killer-verktyget för dev-agenter** ("varför routas detta till premium?").
- `savings_report([window])` → savings/green-sammanfattning (wrappar dashboard-data): sparat $, %, tokens, energi/CO₂e, route-distribution.
- `recent_requests([n], [task_filter])` → senaste request-raderna från historik-loggen (tid, task, risk, modell, tokens, kostnad).
- `get_roster()` → nuvarande roster (tier → modell + provider + pris + enabled) från registry/DB.
- `provider_health()` → provider-hälsa + status.

Inga skriv-/config-verktyg i denna iteration (ändra roster via MCP = senare; känsligare auth/säkerhet).

## Transport & auth

- **HTTP/SSE MCP-endpoint** på den deployade instansen (streamable HTTP eller SSE) för analys-agenter — bakom **samma API-nyckel** som resten (Bearer), eller dashboard-lösenordet. En separat **stdio-binär** (`cmd/mcp` eller liknande) för lokala dev-agenter (Claude Code connectar enkelt via stdio).
- Ingen prompt-text eller secrets läcker: `route_explain` är dry-run (inget provideranrop); loggen innehåller redan inga råa prompts (masking-principen).

## Acceptanskriterier

- MCP-server (Go — helst utan tunga deps; en liten JSON-RPC/MCP-implementation eller en lättviktig lib) som exponerar verktygen ovan med korrekta input-scheman.
- Återanvänd befintliga interna funktioner (engine.Decide för route_explain, dashboard-data-byggaren för savings, history.Recent för loggen, registry/roster för get_roster, health för provider_health) — ingen dubblerad logik.
- Gated bakom auth (API-nyckel/dashboard-lösen). Read-only; rör aldrig fast-path prestanda.
- Kan togglas (t.ex. `ROUTER_MCP_ENABLED` / en egen port eller path). Av som default tills konfigurerad, eller på när nyckel finns — dokumentera.
- Tester för verktygs-handlers (route_explain returnerar rätt beslut; savings/roster/health-former). `make smoke` opåverkad; statiskt CGO_ENABLED=0-bygge intakt.
- Kort docs (`docs/` eller README): hur en agent connectar (stdio + HTTP), vilka verktyg som finns, exempel.

## Tekniska noter

- Håll det litet och additivt — ingen ändring i routing/klassificering. MCP läser bara.
- Om HTTP-MCP: montera på samma mux men egen path (t.ex. `/mcp`) bakom auth; eller separat lyssnare/port.
- Titta på agentanbuds `mcp_http.py`/`mcp_server.py` för transport-val och verktygsform (men implementera i Go här).

## Härkomst

Användarfråga: värde i en MCP-yta för analys-/dev-agenter? Ja. Read-only fast-follow som gör tokenizer agent-native infra. Se [[backlog-mcp-surface]].
