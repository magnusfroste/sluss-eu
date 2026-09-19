BEGIN;

CREATE TABLE IF NOT EXISTS tenants (
  id text PRIMARY KEY,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  retention_days integer NOT NULL DEFAULT 30,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenants_status_check CHECK (status IN ('active', 'disabled')),
  CONSTRAINT tenants_retention_days_check CHECK (retention_days > 0)
);

CREATE TABLE IF NOT EXISTS projects (
  id text PRIMARY KEY,
  tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  name text NOT NULL,
  default_policy_version_id text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
  id text PRIMARY KEY,
  tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
  key_hash text NOT NULL,
  name text NOT NULL,
  scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  CONSTRAINT api_keys_status_check CHECK (status IN ('active', 'disabled', 'revoked')),
  CONSTRAINT api_keys_key_hash_not_plaintext_check CHECK (key_hash ~ '^sha256:[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS model_registry_versions (
  id text PRIMARY KEY,
  version integer NOT NULL UNIQUE,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'draft',
  is_current boolean NOT NULL DEFAULT false,
  checksum text,
  created_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz,
  CONSTRAINT model_registry_versions_status_check CHECK (status IN ('draft', 'active', 'retired')),
  CONSTRAINT model_registry_versions_current_active_check CHECK (
    (is_current = false) OR (status = 'active' AND activated_at IS NOT NULL)
  )
);

CREATE TABLE IF NOT EXISTS providers (
  id text PRIMARY KEY,
  registry_version_id text NOT NULL REFERENCES model_registry_versions(id) ON DELETE RESTRICT,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  base_url text NOT NULL,
  auth_secret_ref text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT providers_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE TABLE IF NOT EXISTS models (
  id text PRIMARY KEY,
  registry_version_id text NOT NULL REFERENCES model_registry_versions(id) ON DELETE RESTRICT,
  provider_id text NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
  provider_model_id text NOT NULL,
  tier text NOT NULL,
  capabilities_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  cost_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_scores_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  latency_profile_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  strengths_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  weaknesses_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT models_tier_check CHECK (tier IN ('local', 'cheap', 'balanced', 'premium', 'specialized')),
  CONSTRAINT models_capabilities_object_check CHECK (jsonb_typeof(capabilities_json) = 'object'),
  CONSTRAINT models_cost_object_check CHECK (jsonb_typeof(cost_json) = 'object'),
  CONSTRAINT models_quality_object_check CHECK (jsonb_typeof(quality_scores_json) = 'object'),
  CONSTRAINT models_latency_object_check CHECK (jsonb_typeof(latency_profile_json) = 'object'),
  CONSTRAINT models_strengths_array_check CHECK (jsonb_typeof(strengths_json) = 'array'),
  CONSTRAINT models_weaknesses_array_check CHECK (jsonb_typeof(weaknesses_json) = 'array'),
  CONSTRAINT models_metadata_object_check CHECK (jsonb_typeof(metadata_json) = 'object'),
  CONSTRAINT models_provider_model_unique UNIQUE (provider_id, provider_model_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_key_hash_uidx
  ON api_keys(key_hash);

CREATE INDEX IF NOT EXISTS projects_tenant_id_idx
  ON projects(tenant_id);

CREATE INDEX IF NOT EXISTS api_keys_tenant_id_idx
  ON api_keys(tenant_id);

CREATE INDEX IF NOT EXISTS api_keys_project_id_idx
  ON api_keys(project_id);

CREATE INDEX IF NOT EXISTS api_keys_active_lookup_idx
  ON api_keys(key_hash)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS model_registry_versions_current_uidx
  ON model_registry_versions(is_current)
  WHERE is_current = true;

CREATE INDEX IF NOT EXISTS model_registry_versions_active_idx
  ON model_registry_versions(status, activated_at DESC)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS providers_status_idx
  ON providers(status);

CREATE INDEX IF NOT EXISTS providers_registry_version_status_idx
  ON providers(registry_version_id, status);

CREATE INDEX IF NOT EXISTS models_provider_id_idx
  ON models(provider_id);

CREATE INDEX IF NOT EXISTS models_enabled_idx
  ON models(enabled);

CREATE INDEX IF NOT EXISTS models_registry_version_enabled_idx
  ON models(registry_version_id, enabled);

COMMIT;
