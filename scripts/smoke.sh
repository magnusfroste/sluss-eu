#!/usr/bin/env bash
# Systematic end-to-end smoke test against the mock provider.
#
# Boots mock-provider + router on local ports, then exercises the full request
# lifecycle (auth, model discovery, routing, provider call, dry-run, metrics,
# dashboard) and asserts each step. Deterministic and credential-free — this is
# the repeatable way to validate "what we have". For real models, run the same
# router with OPENROUTER_API_KEY set and point a client at it.
#
# Usage: make smoke   (or: scripts/smoke.sh)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

KEY="smoke_key"
MOCK_ADDR=":18099"
MOCK_URL="http://localhost:18099"
ROUTER_ADDR=":8099"
BASE="http://localhost:8099"

pass() { echo "  ok: $1"; }
fail() { echo "SMOKE FAIL: $1" >&2; exit 1; }

echo "building binaries..."
go build -o bin/mock-provider ./cmd/mock-provider
go build -o bin/router ./cmd/router

MOCK_PROVIDER_ADDR="$MOCK_ADDR" ./bin/mock-provider >/tmp/smoke-mock.log 2>&1 &
MOCK_PID=$!
LOCAL_API_KEY="$KEY" MOCK_PROVIDER_URL="$MOCK_URL" ROUTER_ADDR="$ROUTER_ADDR" \
  ROUTER_MCP_ENABLED=true \
  ./bin/router >/tmp/smoke-router.log 2>&1 &
ROUTER_PID=$!
cleanup() { kill "$MOCK_PID" "$ROUTER_PID" 2>/dev/null || true; }
trap cleanup EXIT

echo "waiting for router..."
ready=""
for _ in $(seq 1 50); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.2
done
[ -n "$ready" ] || fail "router did not become ready (see /tmp/smoke-router.log)"

echo "running checks..."

# 1. health
curl -fsS "$BASE/healthz" | grep -q '"status":"ok"' || fail "healthz"
pass "healthz"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/readyz")" = "200" ] || fail "readyz"
pass "readyz"

# 2. auth required
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/v1/chat/completions" \
  -d '{"model":"auto","messages":[{"role":"user","content":"hi"}]}')
[ "$code" = "401" ] || fail "chat without key should be 401 (got $code)"
pass "auth enforced (401 without key)"

# 3. model discovery
models=$(curl -fsS "$BASE/v1/models" -H "Authorization: Bearer $KEY")
echo "$models" | grep -q '"id":"cheap-general"' || fail "/v1/models missing cheap-general"
echo "$models" | grep -q '"id":"auto"' || fail "/v1/models missing auto"
pass "/v1/models lists models"

# 4. chat completion + routing header
hdrs=$(curl -s -D - -o /tmp/smoke-chat.json -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -d '{"model":"auto","messages":[{"role":"user","content":"write a git commit message"}]}')
echo "$hdrs" | grep -qi '^HTTP/1.1 200' || fail "chat completion not 200"
echo "$hdrs" | grep -qi '^X-Router-Selected-Model:' || fail "missing X-Router-Selected-Model header"
echo "$hdrs" | grep -qi '^X-Router-Cost-USD:' || fail "missing X-Router-Cost-USD header"
grep -q '"object":"chat.completion"' /tmp/smoke-chat.json || fail "chat response not a completion"
pass "chat completion routed (selected $(echo "$hdrs" | grep -i '^X-Router-Selected-Model:' | tr -d '\r' | awk '{print $2}'))"

# 5. decision dry-run
dec=$(curl -fsS -X POST "$BASE/router/decision" -H "Authorization: Bearer $KEY" \
  -d '{"model":"auto","messages":[{"role":"user","content":"review this auth change for security"}]}')
echo "$dec" | grep -q '"selected_model"' || fail "/router/decision missing selected_model"
pass "/router/decision dry-run"

# 6. metrics
curl -fsS "$BASE/metrics" | grep -q '^router_' || fail "/metrics missing router_ series"
pass "/metrics exposes router series"

# 7. dashboard data (behind auth; the seeded local key is allowed)
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $KEY" "$BASE/router/dashboard/data")" = "200" ] || fail "dashboard data"
pass "dashboard data"

# 8. read-only MCP surface (ISSUE-074): auth-gated, lists tools, route_explain works
mcp_code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/mcp" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
[ "$mcp_code" = "401" ] || fail "/mcp without key should be 401 (got $mcp_code)"
tools=$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
echo "$tools" | grep -q '"route_explain"' || fail "/mcp tools/list missing route_explain"
explain=$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $KEY" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"route_explain","arguments":{"prompt":"review this auth change for security"}}}')
echo "$explain" | grep -q 'selected_model' || fail "/mcp route_explain missing selected_model"
pass "/mcp read-only surface (tools/list + route_explain)"

echo "SMOKE PASS"
