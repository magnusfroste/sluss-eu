# Lokal utveckling

## Komponenter

- Sluss-gatewayn (`cmd/router`) — allt-i-ett: API, admin-UI, policy console, MCP.
- SQLite under `ROUTER_DATA_DIR` (skapas automatiskt) — ingen extern databas
  eller cache behövs.
- Mock provider (`cmd/mock-provider`) — kör helt utan riktiga providernycklar.

## Starta

```bash
cp .env.example .env   # valfritt; funkar med defaults
make dev               # go run ./cmd/router — lyssnar på :8080
make run-mock          # mock-provider i ett annat skal (valfritt)
```

Med `OPENROUTER_API_KEY` satt byggs modellregistret från SQLite-rostern
(Models-sidan); utan nyckel används mock/default-snapshot.

## Testa

```bash
make test              # hela sviten (race)
make test-policy       # policy golden cases
make test-eval         # eval smoke
make lint              # vet + gofmt
```

## Lokal testrequest

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer local_router_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Write a commit message for this diff"}]
  }'
```

## Dry-run av routingbeslut

Utan provideranrop:

```bash
curl -X POST http://localhost:8080/router/decision \
  -H "Authorization: Bearer local_router_key" \
  -d '{"messages":[{"role":"user","content":"Summarise this contract"}]}'
# eller: go run ./cmd/routerctl -prompt "Summarise this contract"
```

Samma dry-run finns i policy-konsolen (`POST /router/policy/dryrun`) och som
MCP-verktyget `route_explain`.

## Vanliga fel

- Modell utan konfigurerad nyckel-env för sin provider är inte routbar —
  kontrollera Models-sidans Test-knapp.
- `make demo-ciso` kör hela CISO-demoflödet mot mock och är den snabbaste
  end-to-end-röken.
