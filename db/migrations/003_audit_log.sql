BEGIN;

-- audit_log: security-relevant control-plane changes and blocked requests
-- (ISSUE-044). Append-only; rows are never updated in place. Retention is
-- governed per tenant by the cleanup job (ISSUE-045).
CREATE TABLE IF NOT EXISTS audit_log (
  id          text        PRIMARY KEY,
  action      text        NOT NULL, -- 'policy.reload' | 'api_key.add' | 'api_key.disable' | 'request.blocked'
  actor       text        NOT NULL DEFAULT '',
  tenant_id   text        NOT NULL DEFAULT '',
  project_id  text        NOT NULL DEFAULT '',
  target      text        NOT NULL DEFAULT '',
  outcome     text        NOT NULL DEFAULT 'success',
  request_id  text        NOT NULL DEFAULT '',
  reason      text        NOT NULL DEFAULT '',
  detail_json jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_log_action_not_empty CHECK (action <> ''),
  CONSTRAINT audit_log_outcome_check     CHECK (outcome IN ('success', 'failure', 'blocked')),
  CONSTRAINT audit_log_detail_object_check CHECK (jsonb_typeof(detail_json) = 'object')
);

CREATE INDEX IF NOT EXISTS audit_log_tenant_created_idx
  ON audit_log(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS audit_log_action_created_idx
  ON audit_log(action, created_at DESC);

CREATE INDEX IF NOT EXISTS audit_log_request_id_idx
  ON audit_log(request_id)
  WHERE request_id <> '';

COMMIT;
