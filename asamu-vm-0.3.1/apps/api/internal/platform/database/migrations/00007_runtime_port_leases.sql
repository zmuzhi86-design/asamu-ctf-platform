-- +goose Up
CREATE TABLE runtime_port_pool (
  worker_id text NOT NULL,
  protocol text NOT NULL CHECK(protocol IN ('tcp','udp')),
  host_port integer NOT NULL CHECK(host_port BETWEEN 1024 AND 65535),
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(worker_id, protocol, host_port)
);

CREATE TABLE runtime_port_leases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  worker_id text NOT NULL,
  instance_id uuid NOT NULL REFERENCES challenge_instances(id) ON DELETE CASCADE,
  operation_id uuid REFERENCES runtime_operations(id) ON DELETE SET NULL,
  protocol text NOT NULL CHECK(protocol IN ('tcp','udp')),
  host_port integer NOT NULL CHECK(host_port BETWEEN 1024 AND 65535),
  internal_port integer NOT NULL CHECK(internal_port BETWEEN 1 AND 65535),
  status text NOT NULL DEFAULT 'reserved' CHECK(status IN ('reserved','active','releasing','released','expired','conflict')),
  lease_token uuid NOT NULL DEFAULT gen_random_uuid(),
  reserved_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz,
  renewed_at timestamptz,
  expires_at timestamptz NOT NULL,
  released_at timestamptz,
  last_error_code text NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX runtime_port_leases_active_port_uq
  ON runtime_port_leases(worker_id, protocol, host_port)
  WHERE status IN ('reserved','active','releasing');
CREATE UNIQUE INDEX runtime_port_leases_active_instance_uq
  ON runtime_port_leases(instance_id, protocol)
  WHERE status IN ('reserved','active','releasing');
CREATE INDEX runtime_port_leases_expiry_idx
  ON runtime_port_leases(status, expires_at)
  WHERE status IN ('reserved','active');

-- +goose Down
-- Expand-and-contract migration: intentionally preserve lease history.
SELECT 1;
