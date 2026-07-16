-- +goose Up
CREATE TABLE hint_unlocks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_id uuid NOT NULL REFERENCES challenges(id) ON DELETE RESTRICT,
  challenge_revision_id uuid NOT NULL REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  competition_id uuid REFERENCES competitions(id) ON DELETE RESTRICT,
  competition_snapshot_id uuid REFERENCES competition_snapshots(id) ON DELETE RESTRICT,
  hint_index integer NOT NULL CHECK(hint_index >= 0),
  owner_scope text NOT NULL CHECK(owner_scope IN ('user','team')),
  owner_id uuid NOT NULL,
  requested_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  cost integer NOT NULL DEFAULT 0 CHECK(cost >= 0),
  score_event_id uuid REFERENCES score_events(id) ON DELETE RESTRICT,
  unlocked_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX hint_unlocks_scope_uq ON hint_unlocks(
  challenge_revision_id,hint_index,owner_scope,owner_id,
  coalesce(competition_id,'00000000-0000-0000-0000-000000000000'::uuid)
);
CREATE INDEX hint_unlocks_owner_idx ON hint_unlocks(owner_scope,owner_id,unlocked_at DESC);

-- +goose Down
-- Hint and score history is intentionally retained.
SELECT 1;
