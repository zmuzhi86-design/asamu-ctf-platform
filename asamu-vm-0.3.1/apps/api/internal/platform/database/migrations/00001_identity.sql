-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email citext NOT NULL, username citext NOT NULL, password_hash text NOT NULL, status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','pending','banned','deleted')), email_verified_at timestamptz, last_login_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX users_email_active_uq ON users(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_username_active_uq ON users(username) WHERE deleted_at IS NULL;
CREATE TABLE user_profiles (user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, display_name text NOT NULL DEFAULT '', bio text NOT NULL DEFAULT '', organization_name text NOT NULL DEFAULT '', avatar_asset_key text NOT NULL DEFAULT '', character_asset_key text NOT NULL DEFAULT '', signature text NOT NULL DEFAULT '', skills jsonb NOT NULL DEFAULT '[]', privacy jsonb NOT NULL DEFAULT '{}', updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE organizations (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), slug citext NOT NULL UNIQUE, name text NOT NULL, type text NOT NULL DEFAULT 'school', description text NOT NULL DEFAULT '', owner_id uuid REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE organization_members (organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE, user_id uuid REFERENCES users(id) ON DELETE CASCADE, role text NOT NULL DEFAULT 'member', joined_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(organization_id,user_id));
CREATE TABLE roles (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), key text NOT NULL UNIQUE, name text NOT NULL, description text NOT NULL DEFAULT '');
CREATE TABLE permissions (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), key text NOT NULL UNIQUE, name text NOT NULL, description text NOT NULL DEFAULT '');
CREATE TABLE user_roles (user_id uuid REFERENCES users(id) ON DELETE CASCADE, role_id uuid REFERENCES roles(id) ON DELETE CASCADE, PRIMARY KEY(user_id,role_id));
CREATE TABLE role_permissions (role_id uuid REFERENCES roles(id) ON DELETE CASCADE, permission_id uuid REFERENCES permissions(id) ON DELETE CASCADE, PRIMARY KEY(role_id,permission_id));
CREATE TABLE refresh_tokens (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, family_id uuid NOT NULL, expires_at timestamptz NOT NULL, used_at timestamptz, revoked_at timestamptz, replaced_by_id uuid, created_at timestamptz NOT NULL DEFAULT now(), ip inet, user_agent text NOT NULL DEFAULT '');
CREATE INDEX refresh_tokens_user_active_idx ON refresh_tokens(user_id,expires_at) WHERE revoked_at IS NULL;
CREATE TABLE password_reset_tokens (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE email_verification_tokens (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE login_records (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid REFERENCES users(id) ON DELETE SET NULL, email citext NOT NULL, success boolean NOT NULL, reason text NOT NULL DEFAULT '', ip inet, user_agent text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE user_bans (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id), reason text NOT NULL, starts_at timestamptz NOT NULL DEFAULT now(), ends_at timestamptz, created_by uuid NOT NULL REFERENCES users(id), revoked_at timestamptz, revoked_by uuid REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now());

-- +goose Down
DROP TABLE IF EXISTS user_bans,login_records,email_verification_tokens,password_reset_tokens,refresh_tokens,role_permissions,user_roles,permissions,roles,organization_members,organizations,user_profiles,users CASCADE;
