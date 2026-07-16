-- +goose Up
CREATE TABLE email_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  recipient citext NOT NULL,
  template_key text NOT NULL CHECK (template_key IN ('verify_email','reset_password','change_email','team_invitation')),
  payload_ciphertext bytea NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed','dead')),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at timestamptz NOT NULL DEFAULT now(),
  locked_at timestamptz,
  sent_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_outbox_dispatch_idx ON email_outbox(status,available_at,locked_at,created_at) WHERE status IN ('pending','failed','sending');
CREATE INDEX password_reset_tokens_user_created_idx ON password_reset_tokens(user_id,created_at DESC);
CREATE INDEX email_verification_tokens_user_created_idx ON email_verification_tokens(user_id,created_at DESC);
ALTER TABLE email_verification_tokens ADD COLUMN purpose text NOT NULL DEFAULT 'verify_email' CHECK (purpose IN ('verify_email','change_email'));
ALTER TABLE email_verification_tokens ADD COLUMN target_email citext;

-- +goose Down
DROP TABLE IF EXISTS email_outbox;
DROP INDEX IF EXISTS password_reset_tokens_user_created_idx;
DROP INDEX IF EXISTS email_verification_tokens_user_created_idx;
ALTER TABLE email_verification_tokens DROP COLUMN IF EXISTS target_email;
ALTER TABLE email_verification_tokens DROP COLUMN IF EXISTS purpose;
