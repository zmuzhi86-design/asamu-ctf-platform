-- +goose Up
ALTER TABLE users
  ADD COLUMN token_version integer NOT NULL DEFAULT 1,
  ADD COLUMN password_changed_at timestamptz,
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false,
  ADD COLUMN pending_email citext,
  ADD COLUMN pending_email_requested_at timestamptz;

CREATE UNIQUE INDEX users_pending_email_active_uq
  ON users(pending_email)
  WHERE pending_email IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE user_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash text NOT NULL UNIQUE,
  token_version integer NOT NULL,
  ip inet,
  user_agent text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  revoke_reason text NOT NULL DEFAULT ''
);
CREATE INDEX user_sessions_user_active_idx
  ON user_sessions(user_id, expires_at)
  WHERE revoked_at IS NULL;

CREATE TABLE platform_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_key text NOT NULL UNIQUE,
  platform_name text NOT NULL,
  short_name text NOT NULL DEFAULT '',
  slogan text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  logo_asset_key text NOT NULL DEFAULT '',
  favicon_asset_key text NOT NULL DEFAULT '',
  footer_markdown text NOT NULL DEFAULT '',
  contact_json jsonb NOT NULL DEFAULT '{}',
  default_locale text NOT NULL DEFAULT 'zh-CN',
  timezone text NOT NULL DEFAULT 'Asia/Shanghai',
  homepage_title text NOT NULL DEFAULT '',
  default_theme_key text NOT NULL DEFAULT 'platform-default',
  default_background_key text NOT NULL DEFAULT '',
  runtime_defaults_json jsonb NOT NULL DEFAULT '{}',
  status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','archived')),
  version integer NOT NULL DEFAULT 1,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);

CREATE TABLE platform_feature_flags (
  profile_id uuid NOT NULL REFERENCES platform_profiles(id) ON DELETE CASCADE,
  feature_key text NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  config_json jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(profile_id, feature_key)
);

CREATE TABLE navigation_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES platform_profiles(id) ON DELETE CASCADE,
  item_key text NOT NULL,
  label text NOT NULL,
  href text NOT NULL,
  icon_asset_key text NOT NULL DEFAULT '',
  required_feature text NOT NULL DEFAULT '',
  required_permission text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  enabled boolean NOT NULL DEFAULT true,
  UNIQUE(profile_id, item_key)
);

CREATE TABLE homepage_blocks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES platform_profiles(id) ON DELETE CASCADE,
  block_key text NOT NULL,
  block_type text NOT NULL,
  title text NOT NULL DEFAULT '',
  config_json jsonb NOT NULL DEFAULT '{}',
  sort_order integer NOT NULL DEFAULT 0,
  enabled boolean NOT NULL DEFAULT true,
  UNIQUE(profile_id, block_key)
);

CREATE TABLE platform_setting_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES platform_profiles(id) ON DELETE RESTRICT,
  version integer NOT NULL,
  snapshot_json jsonb NOT NULL,
  status text NOT NULL CHECK(status IN ('draft','published','rolled_back','archived')),
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  UNIQUE(profile_id, version)
);

CREATE TABLE challenge_directions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug citext NOT NULL UNIQUE,
  name text NOT NULL,
  subtitle text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  icon_asset_key text NOT NULL DEFAULT '',
  card_asset_key text NOT NULL DEFAULT '',
  banner_asset_key text NOT NULL DEFAULT '',
  background_asset_key text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled','archived')),
  show_on_home boolean NOT NULL DEFAULT true,
  show_on_library_header boolean NOT NULL DEFAULT true,
  show_on_library_sidebar boolean NOT NULL DEFAULT true,
  featured boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO challenge_directions (
  id, slug, name, description, card_asset_key, sort_order, status,
  show_on_home, show_on_library_header, show_on_library_sidebar, featured
)
SELECT
  id, key, name, description, scene_asset_key, sort_order,
  CASE WHEN enabled THEN 'active' ELSE 'disabled' END,
  enabled, enabled, enabled, false
FROM challenge_categories
ON CONFLICT (id) DO NOTHING;

ALTER TABLE challenges ADD COLUMN direction_id uuid;
UPDATE challenges SET direction_id = category_id WHERE direction_id IS NULL;
ALTER TABLE challenges
  ADD CONSTRAINT challenges_direction_fk
  FOREIGN KEY(direction_id) REFERENCES challenge_directions(id) ON DELETE RESTRICT;
CREATE INDEX challenges_direction_idx ON challenges(direction_id, status, difficulty);

-- +goose Down
-- Expand-and-contract migration: intentionally preserve all added data on rollback.
SELECT 1;
