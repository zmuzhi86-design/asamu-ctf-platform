-- +goose Up
-- Normalize the immutable registration roster so every competition user has a
-- single authoritative team. The JSON snapshot remains the audit copy.
ALTER TABLE competition_participants
  ADD CONSTRAINT competition_participants_id_competition_uq
  UNIQUE (id,competition_id);

CREATE TABLE competition_roster_members (
  competition_id uuid NOT NULL,
  participant_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  username_snapshot text NOT NULL DEFAULT '',
  role_snapshot text NOT NULL DEFAULT 'member',
  registered_at timestamptz NOT NULL,
  PRIMARY KEY (participant_id,user_id),
  UNIQUE (competition_id,user_id),
  FOREIGN KEY (participant_id,competition_id)
    REFERENCES competition_participants(id,competition_id) ON DELETE CASCADE
);

INSERT INTO competition_roster_members(
  competition_id,participant_id,user_id,username_snapshot,role_snapshot,registered_at
)
SELECT
  cp.competition_id,cp.id,cp.user_id,u.username,'individual',cp.registered_at
FROM competition_participants cp
JOIN users u ON u.id=cp.user_id
WHERE cp.team_id IS NULL AND cp.status='registered';

-- A uniqueness violation here indicates historically overlapping frozen team
-- rosters. The migration intentionally refuses to choose a team silently.
INSERT INTO competition_roster_members(
  competition_id,participant_id,user_id,username_snapshot,role_snapshot,registered_at
)
SELECT
  cp.competition_id,
  cp.id,
  (member->>'userId')::uuid,
  COALESCE(member->>'Username',member->>'username',''),
  COALESCE(member->>'Role',member->>'role','member'),
  cp.registered_at
FROM competition_participants cp
CROSS JOIN LATERAL jsonb_array_elements(cp.roster_snapshot) AS member
WHERE cp.team_id IS NOT NULL
  AND cp.status='registered'
  AND member ? 'userId'
  AND member->>'userId' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$';

CREATE INDEX competition_roster_members_team_idx
  ON competition_roster_members(competition_id,participant_id,user_id);

-- A team competition awards each challenge exactly once, regardless of which
-- roster member submits the winning flag. The claim is created before score
-- events and points are written, so this primary key is the database-level
-- concurrency guard. Existing duplicate history is retained; the earliest
-- solve becomes the canonical claim.
CREATE TABLE team_competition_solve_claims (
  competition_id uuid NOT NULL REFERENCES competitions(id) ON DELETE RESTRICT,
  team_id uuid NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
  challenge_id uuid NOT NULL REFERENCES challenges(id) ON DELETE RESTRICT,
  solve_record_id uuid UNIQUE REFERENCES solve_records(id) ON DELETE RESTRICT,
  claimed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (competition_id,team_id,challenge_id)
);

INSERT INTO team_competition_solve_claims(
  competition_id,team_id,challenge_id,solve_record_id,claimed_at
)
SELECT DISTINCT ON (competition_id,team_id,challenge_id)
  competition_id,team_id,challenge_id,id,solved_at
FROM solve_records
WHERE competition_id IS NOT NULL AND team_id IS NOT NULL
ORDER BY competition_id,team_id,challenge_id,solved_at,id
ON CONFLICT DO NOTHING;

CREATE INDEX team_competition_solve_claims_challenge_idx
  ON team_competition_solve_claims(competition_id,challenge_id)
  WHERE solve_record_id IS NOT NULL;

-- +goose Down
-- Canonical scoring claims and their links to append-only solve history are
-- intentionally retained across application rollbacks.
SELECT 1;
