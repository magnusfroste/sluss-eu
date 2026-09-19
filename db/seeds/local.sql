BEGIN;

INSERT INTO tenants (id, name, status, retention_days)
VALUES ('tenant_local', 'Local Development Tenant', 'active', 30)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  status = EXCLUDED.status,
  retention_days = EXCLUDED.retention_days;

INSERT INTO projects (id, tenant_id, name, default_policy_version_id)
VALUES ('project_local', 'tenant_local', 'Local Development Project', NULL)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  name = EXCLUDED.name,
  default_policy_version_id = EXCLUDED.default_policy_version_id;

INSERT INTO api_keys (id, tenant_id, project_id, key_hash, name, scopes, status)
VALUES (
  'api_key_local',
  'tenant_local',
  'project_local',
  'sha256:4670779d9e85bc75c5dd151ae15395c4a2221068d8ef4250cb967110e6629850',
  'Local development API key',
  ARRAY['chat:completions', 'router:decision', 'router:outcomes'],
  'active'
)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  project_id = EXCLUDED.project_id,
  key_hash = EXCLUDED.key_hash,
  name = EXCLUDED.name,
  scopes = EXCLUDED.scopes,
  status = EXCLUDED.status;

INSERT INTO model_registry_versions (id, version, name, status, is_current, checksum, activated_at)
VALUES (
  'registry_local_v1',
  1,
  'Local registry v1',
  'active',
  true,
  'local-v1',
  now()
)
ON CONFLICT (id) DO UPDATE SET
  version = EXCLUDED.version,
  name = EXCLUDED.name,
  status = EXCLUDED.status,
  is_current = EXCLUDED.is_current,
  checksum = EXCLUDED.checksum,
  activated_at = COALESCE(model_registry_versions.activated_at, EXCLUDED.activated_at);

INSERT INTO providers (id, registry_version_id, name, status, base_url, auth_secret_ref)
VALUES
  ('provider_openai_local', 'registry_local_v1', 'OpenAI Local Fixture', 'active', 'https://api.openai.com/v1', 'env:OPENAI_API_KEY'),
  ('provider_mock_local', 'registry_local_v1', 'Mock Provider', 'active', 'http://localhost:18080/v1', 'env:MOCK_PROVIDER_API_KEY')
ON CONFLICT (id) DO UPDATE SET
  registry_version_id = EXCLUDED.registry_version_id,
  name = EXCLUDED.name,
  status = EXCLUDED.status,
  base_url = EXCLUDED.base_url,
  auth_secret_ref = EXCLUDED.auth_secret_ref,
  updated_at = now();

INSERT INTO models (
  id,
  registry_version_id,
  provider_id,
  provider_model_id,
  tier,
  capabilities_json,
  cost_json,
  quality_scores_json,
  latency_profile_json,
  strengths_json,
  weaknesses_json,
  metadata_json,
  enabled
)
VALUES
  (
    'balanced-coder',
    'registry_local_v1',
    'provider_openai_local',
    'gpt-4.1-mini',
    'balanced',
    '{"chat": true, "streaming": true, "tool_calls": true, "json_schema": true, "vision": false, "long_context": true, "context_window_tokens": 128000}'::jsonb,
    '{"currency": "USD", "unit": "1M tokens", "input_per_million": 0.40, "output_per_million": 1.60}'::jsonb,
    '{"simple_code_edit": 0.78, "hard_code_debugging": 0.55, "security_review": 0.48}'::jsonb,
    '{"p50_first_token_ms": 700, "p95_first_token_ms": 1800}'::jsonb,
    '["code", "summarization", "tool_use"]'::jsonb,
    '["hard_reasoning", "security_review"]'::jsonb,
    '{"profile": "balanced-coder"}'::jsonb,
    true
  ),
  (
    'local-fast-chat',
    'registry_local_v1',
    'provider_mock_local',
    'mock-fast-chat',
    'local',
    '{"chat": true, "streaming": false, "tool_calls": false, "json_schema": true, "vision": false, "long_context": false, "context_window_tokens": 8192}'::jsonb,
    '{"currency": "USD", "unit": "1M tokens", "input_per_million": 0.00, "output_per_million": 0.00}'::jsonb,
    '{"simple_code_edit": 0.35, "hard_code_debugging": 0.10, "security_review": 0.05}'::jsonb,
    '{"p50_first_token_ms": 25, "p95_first_token_ms": 60}'::jsonb,
    '["local_dev", "smoke_tests"]'::jsonb,
    '["hard_reasoning", "production_quality"]'::jsonb,
    '{"profile": "local-fast-chat"}'::jsonb,
    true
  )
ON CONFLICT (id) DO UPDATE SET
  registry_version_id = EXCLUDED.registry_version_id,
  provider_id = EXCLUDED.provider_id,
  provider_model_id = EXCLUDED.provider_model_id,
  tier = EXCLUDED.tier,
  capabilities_json = EXCLUDED.capabilities_json,
  cost_json = EXCLUDED.cost_json,
  quality_scores_json = EXCLUDED.quality_scores_json,
  latency_profile_json = EXCLUDED.latency_profile_json,
  strengths_json = EXCLUDED.strengths_json,
  weaknesses_json = EXCLUDED.weaknesses_json,
  metadata_json = EXCLUDED.metadata_json,
  enabled = EXCLUDED.enabled,
  updated_at = now();

COMMIT;
