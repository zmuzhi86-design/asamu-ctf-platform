-- +goose Up
ALTER TABLE challenge_files ADD COLUMN archived_at timestamptz;
CREATE INDEX challenge_files_active_idx ON challenge_files(challenge_id,created_at) WHERE archived_at IS NULL;

-- +goose Down
-- Published attachment history is intentionally retained.
SELECT 1;
