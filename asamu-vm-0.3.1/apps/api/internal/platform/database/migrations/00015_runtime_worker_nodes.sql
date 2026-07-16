-- +goose Up
CREATE TABLE runtime_worker_nodes (
  worker_id text PRIMARY KEY,
  hostname text NOT NULL,
  status text NOT NULL DEFAULT 'online' CHECK(status IN ('online','offline')),
  enabled boolean NOT NULL DEFAULT true,
  draining boolean NOT NULL DEFAULT false,
  cpu_total_milli integer NOT NULL CHECK(cpu_total_milli > 0),
  memory_total_mb integer NOT NULL CHECK(memory_total_mb > 0),
  max_instances integer NOT NULL CHECK(max_instances > 0),
  active_instances integer NOT NULL DEFAULT 0 CHECK(active_instances >= 0),
  reserved_cpu_milli integer NOT NULL DEFAULT 0 CHECK(reserved_cpu_milli >= 0),
  reserved_memory_mb integer NOT NULL DEFAULT 0 CHECK(reserved_memory_mb >= 0),
  supported_protocols jsonb NOT NULL DEFAULT '["http","tcp","udp"]',
  cached_images jsonb NOT NULL DEFAULT '[]',
  last_error_code text NOT NULL DEFAULT '',
  last_heartbeat timestamptz NOT NULL DEFAULT now(),
  registered_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1
);
CREATE INDEX runtime_worker_nodes_schedule_idx
  ON runtime_worker_nodes(enabled,draining,status,last_heartbeat,active_instances);

-- +goose Down
-- Expand-only runtime migration: preserve worker history and operator drain state.
SELECT 1;
