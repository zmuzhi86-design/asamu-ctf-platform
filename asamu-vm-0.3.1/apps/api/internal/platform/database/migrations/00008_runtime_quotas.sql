-- +goose Up
CREATE TABLE runtime_quota_policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type text NOT NULL CHECK(scope_type IN ('platform','worker','challenge','competition','team','user')),
  scope_id uuid,
  max_active_instances integer NOT NULL CHECK(max_active_instances >= 0),
  max_cpu_milli integer NOT NULL CHECK(max_cpu_milli > 0),
  max_memory_mb integer NOT NULL CHECK(max_memory_mb > 0),
  max_pids integer NOT NULL CHECK(max_pids > 0),
  max_ttl_seconds integer NOT NULL CHECK(max_ttl_seconds > 0),
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX runtime_quota_policies_scope_uq
  ON runtime_quota_policies(scope_type, coalesce(scope_id,'00000000-0000-0000-0000-000000000000'::uuid));

CREATE TABLE runtime_quota_overrides (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type text NOT NULL CHECK(scope_type IN ('worker','challenge','competition','team','user')),
  scope_id uuid NOT NULL,
  max_active_instances integer,
  max_cpu_milli integer,
  max_memory_mb integer,
  max_pids integer,
  max_ttl_seconds integer,
  reason text NOT NULL,
  starts_at timestamptz NOT NULL DEFAULT now(),
  ends_at timestamptz,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX runtime_quota_overrides_active_idx
  ON runtime_quota_overrides(scope_type, scope_id, starts_at, ends_at);

CREATE TABLE runtime_usage_counters (
  owner_scope text NOT NULL CHECK(owner_scope IN ('user','team')),
  owner_id uuid NOT NULL,
  day date NOT NULL,
  starts integer NOT NULL DEFAULT 0,
  active_instances integer NOT NULL DEFAULT 0,
  reserved_cpu_milli bigint NOT NULL DEFAULT 0,
  reserved_memory_mb bigint NOT NULL DEFAULT 0,
  runtime_seconds bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(owner_scope, owner_id, day)
);

INSERT INTO runtime_quota_policies(scope_type,max_active_instances,max_cpu_milli,max_memory_mb,max_pids,max_ttl_seconds)
VALUES
  ('platform',100000,4000,8192,1024,14400),
  ('user',2,500,512,256,14400),
  ('team',5,1000,1024,512,14400)
ON CONFLICT DO NOTHING;

-- +goose Down
-- Expand-and-contract migration: intentionally preserve quota history.
SELECT 1;
