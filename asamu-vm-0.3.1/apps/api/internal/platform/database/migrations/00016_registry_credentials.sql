-- +goose Up
CREATE TABLE registry_credentials (
  id uuid PRIMARY KEY,
  name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
  registry_host text NOT NULL UNIQUE,
  username text NOT NULL CHECK(length(username) BETWEEN 1 AND 200),
  encrypted_token bytea NOT NULL,
  token_fingerprint text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  last_used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1
);

ALTER TABLE challenge_runtime_configs
  ADD COLUMN registry_credential_id uuid REFERENCES registry_credentials(id) ON DELETE RESTRICT;
ALTER TABLE challenge_runtime_revisions
  ADD COLUMN registry_credential_id uuid REFERENCES registry_credentials(id) ON DELETE RESTRICT;
CREATE INDEX challenge_runtime_configs_registry_credential_idx
  ON challenge_runtime_configs(registry_credential_id) WHERE registry_credential_id IS NOT NULL;
CREATE INDEX challenge_runtime_revisions_registry_credential_idx
  ON challenge_runtime_revisions(registry_credential_id) WHERE registry_credential_id IS NOT NULL;

-- +goose Down
-- Expand-only security migration: credentials and immutable revision references are retained.
SELECT 1;
