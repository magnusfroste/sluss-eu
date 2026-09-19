BEGIN;

-- request_logs: one row per routed request with decision metadata.
CREATE TABLE IF NOT EXISTS request_logs (
  id               text        PRIMARY KEY,
  tenant_id        text        NOT NULL,
  project_id       text        NOT NULL DEFAULT '',
  task_type        text        NOT NULL,
  risk_level       text        NOT NULL,
  sensitivity      text        NOT NULL DEFAULT 'none',
  selected_model   text        NOT NULL,
  selected_provider text       NOT NULL,
  policy_version   text        NOT NULL DEFAULT '',
  prompt_tokens    integer     NOT NULL DEFAULT 0,
  estimated_cost_usd numeric(14,8) NOT NULL DEFAULT 0,
  routing_duration_ms integer  NOT NULL DEFAULT 0,
  blocked          boolean     NOT NULL DEFAULT false,
  block_code       text        NOT NULL DEFAULT '',
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT request_logs_task_type_not_empty   CHECK (task_type   <> ''),
  CONSTRAINT request_logs_risk_level_not_empty  CHECK (risk_level  <> ''),
  CONSTRAINT request_logs_prompt_tokens_gte0    CHECK (prompt_tokens >= 0),
  CONSTRAINT request_logs_routing_duration_gte0 CHECK (routing_duration_ms >= 0)
);

-- route_attempts: one row per provider call (primary + each fallback attempt).
CREATE TABLE IF NOT EXISTS route_attempts (
  id              text        PRIMARY KEY,
  request_id      text        NOT NULL,
  provider_id     text        NOT NULL,
  model_id        text        NOT NULL,
  attempt_index   integer     NOT NULL DEFAULT 0,
  success         boolean     NOT NULL,
  error_code      text        NOT NULL DEFAULT '',
  duration_ms     integer     NOT NULL DEFAULT 0,
  input_tokens    integer     NOT NULL DEFAULT 0,
  output_tokens   integer     NOT NULL DEFAULT 0,
  actual_cost_usd numeric(14,8) NOT NULL DEFAULT 0,
  first_token_ms  integer     NOT NULL DEFAULT 0,
  attempted_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT route_attempts_attempt_index_gte0 CHECK (attempt_index >= 0),
  CONSTRAINT route_attempts_duration_gte0      CHECK (duration_ms >= 0),
  CONSTRAINT route_attempts_input_tokens_gte0  CHECK (input_tokens >= 0),
  CONSTRAINT route_attempts_output_tokens_gte0 CHECK (output_tokens >= 0)
);

-- spend_aggregations: pre-aggregated spend for fast dashboard queries.
CREATE TABLE IF NOT EXISTS spend_aggregations (
  id          text    PRIMARY KEY,
  tenant_id   text    NOT NULL,
  model_id    text    NOT NULL,
  period      text    NOT NULL, -- 'daily' | 'monthly'
  period_key  text    NOT NULL, -- e.g. '2026-06-04' or '2026-06'
  requests    integer NOT NULL DEFAULT 0,
  input_tokens  bigint NOT NULL DEFAULT 0,
  output_tokens bigint NOT NULL DEFAULT 0,
  cost_usd    numeric(14,8) NOT NULL DEFAULT 0,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT spend_aggregations_tenant_model_period UNIQUE (tenant_id, model_id, period, period_key)
);

CREATE INDEX IF NOT EXISTS request_logs_tenant_created_idx
  ON request_logs(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS request_logs_model_idx
  ON request_logs(selected_model);

CREATE INDEX IF NOT EXISTS route_attempts_request_id_idx
  ON route_attempts(request_id);

CREATE INDEX IF NOT EXISTS route_attempts_provider_model_idx
  ON route_attempts(provider_id, model_id, attempted_at DESC);

CREATE INDEX IF NOT EXISTS spend_aggregations_tenant_period_idx
  ON spend_aggregations(tenant_id, period, period_key);

COMMIT;
