-- +goose Up
UPDATE users
SET pending_email = NULL
WHERE pending_email = '';

DROP INDEX IF EXISTS users_pending_email_active_uq;
CREATE UNIQUE INDEX users_pending_email_active_uq
  ON users(pending_email)
  WHERE pending_email IS NOT NULL
    AND pending_email <> ''
    AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS users_pending_email_active_uq;
CREATE UNIQUE INDEX users_pending_email_active_uq
  ON users(pending_email)
  WHERE pending_email IS NOT NULL
    AND deleted_at IS NULL;
