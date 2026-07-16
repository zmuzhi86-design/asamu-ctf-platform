-- +goose Up
ALTER TABLE score_events
  ADD COLUMN rule_snapshot jsonb NOT NULL DEFAULT '{"legacy":true}',
  ADD COLUMN parent_event_id uuid REFERENCES score_events(id) ON DELETE RESTRICT,
  ADD COLUMN reason text NOT NULL DEFAULT '',
  ADD COLUMN created_by uuid REFERENCES users(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX score_events_single_correction_uq
  ON score_events(parent_event_id)
  WHERE parent_event_id IS NOT NULL;

-- +goose Down
-- Append-only scoring history is intentionally retained.
SELECT 1;
