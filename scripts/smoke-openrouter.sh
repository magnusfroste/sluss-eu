#!/usr/bin/env bash
# Live smoke against OpenRouter — the periodic real-provider counterpart to the
# deterministic mock smoke (scripts/smoke.sh).
#
# Skips cleanly when OPENROUTER_API_KEY is unset (so it is safe to invoke in any
# environment). When the key is set it boots the router against OpenRouter, makes
# a real chat completion and asserts a real response plus a non-zero realized
# cost header.
#
# Usage: make smoke-live   (or: OPENROUTER_API_KEY=sk-or-... scripts/smoke-openrouter.sh)
set -euo pipefail

if [ -z "${OPENROUTER_API_KEY:-}" ]; then
  echo "smoke-live skipped: set OPENROUTER_API_KEY to run a live OpenRouter check."
  exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

KEY="smoke_key"
ROUTER_ADDR=":8097"
BASE="http://localhost:8097"

pass() { echo "  ok: $1"; }
fail() { echo "SMOKE-LIVE FAIL: $1" >&2; exit 1; }

echo "building router..."
go build -o bin/router ./cmd/router

LOCAL_API_KEY="$KEY" ROUTER_ADDR="$ROUTER_ADDR" ./bin/router >/tmp/smoke-live-router.log 2>&1 &
ROUTER_PID=$!
trap 'kill "$ROUTER_PID" 2>/dev/null || true' EXIT

echo "waiting for router..."
ready=""
for _ in $(seq 1 50); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.2
done
[ -n "$ready" ] || fail "router did not become ready (see /tmp/smoke-live-router.log)"

# Confirm the router selected the OpenRouter provider (not the mock fallback).
grep -q '"msg":"using OpenRouter provider"' /tmp/smoke-live-router.log \
  || fail "router did not start with the OpenRouter provider (is OPENROUTER_API_KEY exported?)"
pass "router using OpenRouter provider"

echo "making a real chat completion..."
hdrs=$(curl -s -D - -o /tmp/smoke-live-chat.json --max-time 60 \
  -X POST "$BASE/v1/chat/completions" -H "Authorization: Bearer $KEY" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Reply with the single word: pong"}]}')

echo "$hdrs" | grep -qi '^HTTP/1.1 200' || fail "chat not 200 (body: $(cat /tmp/smoke-live-chat.json))"
model=$(echo "$hdrs" | grep -i '^X-Router-Selected-Model:' | tr -d '\r' | awk '{print $2}')
[ -n "$model" ] || fail "missing X-Router-Selected-Model"
grep -q '"content"' /tmp/smoke-live-chat.json || fail "no content in completion"
pass "real completion (model=$model)"

cost=$(echo "$hdrs" | grep -i '^X-Router-Cost-USD:' | tr -d '\r' | awk '{print $2}')
[ -n "$cost" ] || fail "missing X-Router-Cost-USD"
# Realized cost should be > 0 for a real call with token usage.
awk "BEGIN{exit !($cost > 0)}" || fail "realized cost not > 0 (got $cost)"
pass "realized cost from real usage (\$$cost)"

echo "SMOKE-LIVE PASS"
