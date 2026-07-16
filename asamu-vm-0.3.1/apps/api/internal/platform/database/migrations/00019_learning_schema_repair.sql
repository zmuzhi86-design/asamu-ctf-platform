-- +goose Up
-- Repair installations where an older deployment recorded migration 00018 but
-- the one-shot init container was reused before all learning tables existed.
CREATE TABLE IF NOT EXISTS learning_paths (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug citext NOT NULL UNIQUE,
  direction_id uuid REFERENCES challenge_categories(id) ON DELETE SET NULL,
  title text NOT NULL,
  summary text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  prerequisite text NOT NULL DEFAULT '',
  estimated_minutes int NOT NULL DEFAULT 60 CHECK(estimated_minutes BETWEEN 1 AND 100000),
  hero_asset_key text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','archived')),
  featured boolean NOT NULL DEFAULT false,
  sort_order int NOT NULL DEFAULT 0,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS learning_paths_public_idx ON learning_paths(status,sort_order,title);

CREATE TABLE IF NOT EXISTS learning_stages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  path_id uuid NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  sort_order int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS learning_stages_path_idx ON learning_stages(path_id,sort_order);

CREATE TABLE IF NOT EXISTS learning_stage_challenges (
  stage_id uuid NOT NULL REFERENCES learning_stages(id) ON DELETE CASCADE,
  challenge_id uuid NOT NULL REFERENCES challenges(id) ON DELETE RESTRICT,
  sort_order int NOT NULL DEFAULT 0,
  required boolean NOT NULL DEFAULT true,
  PRIMARY KEY(stage_id,challenge_id)
);
CREATE INDEX IF NOT EXISTS learning_stage_challenges_challenge_idx ON learning_stage_challenges(challenge_id);

-- +goose Down
-- Expand-only repair migration: never drop recovered learning data.
SELECT 1;
