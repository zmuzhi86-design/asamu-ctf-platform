-- +goose Up
ALTER TABLE challenge_instances DROP CONSTRAINT IF EXISTS challenge_instances_status_check;
ALTER TABLE challenge_instances
  ADD CONSTRAINT challenge_instances_status_check CHECK(status IN (
    'pending','pulling','creating','starting','running','restarting','resetting',
    'stopping','stopped','expired','failed','interrupted','deleted'
  ));

ALTER TABLE challenge_instances
  ADD COLUMN status_version bigint NOT NULL DEFAULT 1,
  ADD COLUMN worker_id text NOT NULL DEFAULT '',
  ADD COLUMN operation_id uuid,
  ADD COLUMN runtime_revision_id uuid,
  ADD COLUMN owner_scope text NOT NULL DEFAULT 'user' CHECK(owner_scope IN ('user','team','instance')),
  ADD COLUMN owner_id uuid,
  ADD COLUMN last_error_code text NOT NULL DEFAULT '',
  ADD COLUMN last_error_message text NOT NULL DEFAULT '',
  ADD COLUMN heartbeat_at timestamptz;

UPDATE challenge_instances
SET owner_scope = CASE WHEN owner_team_id IS NULL THEN 'user' ELSE 'team' END,
    owner_id = COALESCE(owner_team_id, owner_user_id),
    last_error_code = error_code
WHERE owner_id IS NULL;

ALTER TABLE challenge_instances ALTER COLUMN owner_id SET NOT NULL;

DROP INDEX IF EXISTS challenge_instances_active_uq;
CREATE UNIQUE INDEX challenge_instances_active_uq
  ON challenge_instances(
    challenge_id,
    owner_scope,
    owner_id,
    coalesce(competition_id,'00000000-0000-0000-0000-000000000000'::uuid)
  )
  WHERE status IN ('pending','pulling','creating','starting','running','restarting','resetting','stopping');

CREATE INDEX challenge_instances_worker_heartbeat_idx
  ON challenge_instances(worker_id, heartbeat_at)
  WHERE status IN ('pulling','creating','starting','running','restarting','resetting','stopping');

CREATE TABLE runtime_operations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key text NOT NULL,
  instance_id uuid NOT NULL REFERENCES challenge_instances(id) ON DELETE CASCADE,
  operation_type text NOT NULL CHECK(operation_type IN ('start','restart','reset','stop','extend','expire','reconcile','delete')),
  requested_by uuid REFERENCES users(id),
  status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','retrying','completed','failed','cancelled')),
  retry_count integer NOT NULL DEFAULT 0,
  max_retries integer NOT NULL DEFAULT 3,
  payload jsonb NOT NULL DEFAULT '{}',
  result jsonb NOT NULL DEFAULT '{}',
  error_code text NOT NULL DEFAULT '',
  error_message text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  UNIQUE(requested_by, idempotency_key)
);
CREATE INDEX runtime_operations_dispatch_idx
  ON runtime_operations(status, created_at)
  WHERE status IN ('pending','retrying');

ALTER TABLE challenge_instances
  ADD CONSTRAINT challenge_instances_operation_fk
  FOREIGN KEY(operation_id) REFERENCES runtime_operations(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE runtime_dead_letters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  operation_id uuid NOT NULL UNIQUE REFERENCES runtime_operations(id) ON DELETE CASCADE,
  stream_message_id text NOT NULL DEFAULT '',
  error_code text NOT NULL,
  error_message text NOT NULL DEFAULT '',
  payload jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  resolved_by uuid REFERENCES users(id)
);

-- +goose Down
-- Expand-and-contract migration: intentionally preserve runtime history.
SELECT 1;
