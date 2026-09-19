.PHONY: build dev test test-unit test-integration test-eval test-policy test-regression eval-report eval-recommend autopilot smoke demo-ciso smoke-live load lint fmt vet tidy clean run run-mock migrate seed

GO ?= go
PSQL ?= psql
DATABASE_URL ?= postgres://tokenizer:tokenizer@localhost:5432/tokenizer?sslmode=disable
BIN_DIR := bin
ROUTER_BIN := $(BIN_DIR)/router
MOCK_BIN := $(BIN_DIR)/mock-provider
WORKER_BIN := $(BIN_DIR)/worker
CTL_BIN := $(BIN_DIR)/routerctl
MCP_BIN := $(BIN_DIR)/mcp

build:
	$(GO) build -o $(ROUTER_BIN) ./cmd/router
	$(GO) build -o $(MOCK_BIN) ./cmd/mock-provider
	$(GO) build -o $(WORKER_BIN) ./cmd/worker
	$(GO) build -o $(CTL_BIN) ./cmd/routerctl
	$(GO) build -o $(MCP_BIN) ./cmd/mcp

run: build
	@set -a; [ -f .env ] && . ./.env || true; set +a; $(ROUTER_BIN)

run-mock: build
	$(MOCK_BIN)

dev:
	@set -a; [ -f .env ] && . ./.env || true; set +a; $(GO) run ./cmd/router

migrate:
	@for f in $$(ls db/migrations/*.sql | sort); do \
		echo "applying $$f"; \
		$(PSQL) "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done

seed:
	$(PSQL) "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f db/seeds/local.sql

test:
	$(GO) test ./... -race -count=1

test-unit:
	$(GO) test ./internal/... -race -count=1

test-integration:
	$(GO) test ./test/integration/... -race -count=1

test-eval:
	$(GO) test ./internal/evals/... -count=1 -run 'TestEvalSmoke|TestDataset' -v

test-policy:
	$(GO) test ./internal/policy/... -count=1 -run 'TestParsePolicyTestCases|TestRunPolicyTests'

test-regression:
	$(GO) test ./internal/evals/... -count=1 -run TestRegressionSuite -v

eval-report:
	$(GO) run ./cmd/eval-report -out eval-report

# Eval report plus per-task-class policy floor recommendations (downgrade/upgrade/
# keep) mined from the measured cost/quality frontier. Writes a reviewable policy
# YAML snippet — never auto-applied. See 06-engineering/01-routing-policy-reference.md.
eval-recommend:
	$(GO) run ./cmd/eval-report -out eval-report -recommend

# Autopilot: run recommendations through hard guardrails and emit an
# auto-approved policy (autopilot-policy.yaml) plus a decision log. Review, then
# adopt with a policy reload — nothing is applied to the live router. See
# docs/autopilot.md.
autopilot:
	$(GO) run ./cmd/eval-report -out eval-report -autopilot

# End-to-end smoke test against the mock provider (boots mock + router, exercises
# the full request lifecycle). Repeatable and credential-free.
smoke:
	./scripts/smoke.sh

# CISO pitch demo (ISSUE-082): the whole value chain end-to-end against the mock
# — provisioning, cloud-when-safe, the visible cloud→local switch on PII,
# fail-closed on a residency requirement, tamper-evident audit verify, control
# report, and ROI. Synthetic data only. See examples/demo-ciso/README.md.
demo-ciso:
	./scripts/demo-ciso.sh

# Live smoke against OpenRouter (periodic real-provider validation). Skips when
# OPENROUTER_API_KEY is unset.
smoke-live:
	./scripts/smoke-openrouter.sh

# Concurrent load test against the mock provider (ISSUE-064). Asserts end-to-end
# p95 under budget with zero errors and prints router_routing_overhead_ms.
# Deterministic and credential-free — the beta-gate latency-under-load check.
load:
	./scripts/load.sh

lint: vet
	@out=$$(gofmt -l . | grep -v '^vendor/' || true); \
	if [ -n "$$out" ]; then echo "gofmt issues:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)
