-- +goose Up
ALTER TABLE challenge_runtime_configs
  ADD COLUMN flag_format text NOT NULL DEFAULT 'standard'
  CHECK (flag_format IN ('standard','uuid'));

ALTER TABLE challenge_runtime_revisions
  ADD COLUMN flag_format text NOT NULL DEFAULT 'standard'
  CHECK (flag_format IN ('standard','uuid'));

ALTER TABLE challenge_runtime_configs
  ALTER COLUMN protocol SET DEFAULT 'tcp';

-- +goose Down
-- Published runtime revisions and their Flag format are intentionally retained.
SELECT 1;
