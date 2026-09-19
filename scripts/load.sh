#!/usr/bin/env bash
# Concurrent load test against the mock provider (ISSUE-064).
#
# Boots mock-provider + router on local ports and drives concurrent traffic
# through /v1/chat/completions, asserting end-to-end p95 under a budget and zero
# errors. Deterministic and credential-free — this measures routing overhead
# under load (the beta-gate latency check), isolating the router from real
# provider variance. The router's own internal overhead is also scraped from
# /metrics (router_routing_overhead_ms).
#
# Usage: make load   (or: scripts/load.sh)
#   Env: LOAD_CONCURRENCY (default 25), LOAD_REQUESTS (default 500),
#        LOAD_P95_BUDGET_MS (default 150)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

KEY="load_key"
MOCK_ADDR=":18098"
MOCK_URL="http://localhost:18098"
ROUTER_ADDR=":8098"
BASE="http://localhost:8098"

CONC="${LOAD_CONCURRENCY:-25}"
REQS="${LOAD_REQUESTS:-500}"
BUDGET="${LOAD_P95_BUDGET_MS:-150}"

echo "building binaries..."
go build -o bin/mock-provider ./cmd/mock-provider
go build -o bin/router ./cmd/router
go build -o bin/loadtest ./cmd/loadtest

MOCK_PROVIDER_ADDR="$MOCK_ADDR" ./bin/mock-provider >/tmp/load-mock.log 2>&1 &
MOCK_PID=$!
# No OPENROUTER_API_KEY → router uses the mock; empty policy path → built-in policy.
LOCAL_API_KEY="$KEY" MOCK_PROVIDER_URL="$MOCK_URL" ROUTER_ADDR="$ROUTER_ADDR" \
  ROUTER_POLICY_PATH="" ROUTER_SHADOW_POLICY_PATH="" \
  ./bin/router >/tmp/load-router.log 2>&1 &
ROUTER_PID=$!
cleanup() { kill "$MOCK_PID" "$ROUTER_PID" 2>/dev/null || true; }
trap cleanup EXIT

echo "waiting for router..."
ready=""
for _ in $(seq 1 50); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.2
done
[ -n "$ready" ] || { echo "LOAD FAIL: router did not become ready (see /tmp/load-router.log)" >&2; exit 1; }

echo "running load: ${CONC} concurrent x ${REQS} requests (p95 budget ${BUDGET}ms)..."
./bin/loadtest -url "$BASE" -key "$KEY" -concurrency "$CONC" -requests "$REQS" -p95-budget-ms "$BUDGET"
rc=$?

echo "--- router internal routing overhead (from /metrics) ---"
curl -fsS "$BASE/metrics" | grep -E '^router_routing_overhead_ms_(bucket|count|sum)' | tail -16 || true

exit $rc
