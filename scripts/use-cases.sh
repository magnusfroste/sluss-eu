#!/usr/bin/env bash
# Runs the use cases in examples/use-cases against a fresh Sluss with test
# providers: "anthropic" plays a model on your own hardware, "openai" a cloud
# model. No key, no network. Prints one line per case, or a Markdown table
# with MARKDOWN=1, and fails if any outcome differs from cases.yaml.
set -euo pipefail
cd "$(dirname "$0")/.."

KEY="usecases_key"
MOCK_ADDR=":18098"; MOCK_URL="http://localhost:18098"
ROUTER_ADDR=":8098"; BASE="http://localhost:8098"
DATA_DIR="$(mktemp -d)"

go build -o bin/router ./cmd/router
go build -o bin/mock-provider ./cmd/mock-provider
go build -o bin/usecases ./cmd/usecases

MOCK_PROVIDER_ADDR="$MOCK_ADDR" ./bin/mock-provider >/tmp/usecases-mock.log 2>&1 &
MOCK_PID=$!
LOCAL_API_KEY="$KEY" MOCK_PROVIDER_URL="$MOCK_URL" ROUTER_ADDR="$ROUTER_ADDR" \
  ROUTER_DATA_DIR="$DATA_DIR" \
  ROUTER_POLICY_PATH="examples/use-cases/policy.yaml" \
  ROUTER_PROVIDER_TAGS="anthropic=local,eu-resident;openai=cloud" \
  ROUTER_UPDATE_CHECK=off ROUTER_LOG_BANNER=off \
  ./bin/router >/tmp/usecases-router.log 2>&1 &
ROUTER_PID=$!
cleanup() { kill "$MOCK_PID" "$ROUTER_PID" 2>/dev/null || true; rm -rf "$DATA_DIR"; }
trap cleanup EXIT

for _ in $(seq 1 50); do
  curl -fsS "$BASE/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done

args=(-url "$BASE" -key "$KEY" -cases examples/use-cases/cases.yaml -local anthropic)
if [ "${MARKDOWN:-}" = "1" ]; then args+=(-markdown); fi
./bin/usecases "${args[@]}"
