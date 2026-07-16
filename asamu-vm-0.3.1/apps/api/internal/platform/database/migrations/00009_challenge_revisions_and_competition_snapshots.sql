-- +goose Up
CREATE TABLE challenge_revisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_id uuid NOT NULL REFERENCES challenges(id) ON DELETE RESTRICT,
  version integer NOT NULL,
  title text NOT NULL,
  summary text NOT NULL DEFAULT '',
  description_markdown text NOT NULL DEFAULT '',
  direction_id uuid REFERENCES challenge_directions(id) ON DELETE RESTRICT,
  category_id uuid NOT NULL REFERENCES challenge_categories(id) ON DELETE RESTRICT,
  difficulty text NOT NULL,
  author_name text NOT NULL DEFAULT '',
  visibility text NOT NULL,
  score_mode text NOT NULL,
  base_score integer NOT NULL,
  minimum_score integer NOT NULL,
  maximum_score integer NOT NULL,
  dynamic_decay integer NOT NULL DEFAULT 0,
  is_dynamic boolean NOT NULL DEFAULT false,
  tags_json jsonb NOT NULL DEFAULT '[]',
  hints_json jsonb NOT NULL DEFAULT '[]',
  knowledge_points_json jsonb NOT NULL DEFAULT '[]',
  published_by uuid REFERENCES users(id),
  published_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(challenge_id, version)
);

CREATE TABLE challenge_runtime_revisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_revision_id uuid NOT NULL UNIQUE REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  image_ref text NOT NULL,
  image_digest text NOT NULL DEFAULT '',
  internal_port integer NOT NULL,
  protocol text NOT NULL,
  cpu_milli integer NOT NULL,
  memory_mb integer NOT NULL,
  pids_limit integer NOT NULL,
  disk_mb integer NOT NULL,
  ttl_seconds integer NOT NULL,
  max_ttl_seconds integer NOT NULL,
  read_only_root_fs boolean NOT NULL,
  environment_template jsonb NOT NULL DEFAULT '{}',
  healthcheck_json jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE challenge_flag_revisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_revision_id uuid NOT NULL REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  source_flag_id uuid REFERENCES challenge_flags(id) ON DELETE SET NULL,
  kind text NOT NULL,
  hmac bytea,
  regex_pattern text NOT NULL DEFAULT '',
  stage integer NOT NULL DEFAULT 1,
  policy_json jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX challenge_flag_revisions_lookup_idx ON challenge_flag_revisions(challenge_revision_id, stage);

CREATE TABLE challenge_file_revisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_revision_id uuid NOT NULL REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  source_file_id uuid REFERENCES challenge_files(id) ON DELETE SET NULL,
  name text NOT NULL,
  object_key text NOT NULL,
  mime_type text NOT NULL,
  sha256 text NOT NULL,
  size bigint NOT NULL,
  public boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE challenges ADD COLUMN current_published_revision_id uuid;

INSERT INTO challenge_revisions(
  id,challenge_id,version,title,summary,description_markdown,direction_id,category_id,
  difficulty,author_name,visibility,score_mode,base_score,minimum_score,maximum_score,
  dynamic_decay,is_dynamic,tags_json,hints_json,knowledge_points_json,published_at,created_at
)
SELECT
  gen_random_uuid(),c.id,1,c.title,c.summary,c.description_markdown,c.direction_id,c.category_id,
  c.difficulty,c.author_name,c.visibility,c.score_mode,c.base_score,c.minimum_score,c.maximum_score,
  c.dynamic_decay,c.is_dynamic,
  COALESCE((SELECT jsonb_agg(t.name ORDER BY t.name) FROM challenge_tag_links l JOIN challenge_tags t ON t.id=l.tag_id WHERE l.challenge_id=c.id),'[]'::jsonb),
  COALESCE((SELECT jsonb_agg(jsonb_build_object('title',h.title,'content',h.content_markdown,'cost',h.cost,'sortOrder',h.sort_order) ORDER BY h.sort_order) FROM challenge_hints h WHERE h.challenge_id=c.id),'[]'::jsonb),
  COALESCE((SELECT jsonb_agg(k.name ORDER BY k.sort_order) FROM challenge_knowledge_points k WHERE k.challenge_id=c.id),'[]'::jsonb),
  COALESCE(c.published_at,c.updated_at),c.created_at
FROM challenges c WHERE c.status='published';

UPDATE challenges c SET current_published_revision_id=r.id
FROM challenge_revisions r WHERE r.challenge_id=c.id AND r.version=1;

ALTER TABLE challenges ADD CONSTRAINT challenges_current_revision_fk
  FOREIGN KEY(current_published_revision_id) REFERENCES challenge_revisions(id) ON DELETE RESTRICT;

INSERT INTO challenge_runtime_revisions(
  id,challenge_revision_id,image_ref,image_digest,internal_port,protocol,cpu_milli,memory_mb,
  pids_limit,disk_mb,ttl_seconds,max_ttl_seconds,read_only_root_fs,environment_template
)
SELECT gen_random_uuid(),r.id,cfg.image_ref,cfg.image_digest,cfg.internal_port,cfg.protocol,
  cfg.cpu_milli,cfg.memory_mb,cfg.pids_limit,cfg.disk_mb,cfg.ttl_seconds,cfg.max_ttl_seconds,
  cfg.read_only_root_fs,cfg.environment_template
FROM challenge_revisions r JOIN challenge_runtime_configs cfg ON cfg.challenge_id=r.challenge_id
WHERE r.version=1 AND cfg.enabled=true;

INSERT INTO challenge_flag_revisions(challenge_revision_id,source_flag_id,kind,hmac,regex_pattern,stage)
SELECT r.id,f.id,f.kind,f.hmac,f.regex_pattern,f.stage
FROM challenge_revisions r JOIN challenge_flags f ON f.challenge_id=r.challenge_id
WHERE r.version=1 AND f.enabled=true;

INSERT INTO challenge_file_revisions(challenge_revision_id,source_file_id,name,object_key,mime_type,sha256,size,public)
SELECT r.id,f.id,f.name,f.object_key,f.mime_type,f.sha256,f.size,f.public
FROM challenge_revisions r JOIN challenge_files f ON f.challenge_id=r.challenge_id
WHERE r.version=1;

CREATE TABLE competition_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  competition_id uuid NOT NULL REFERENCES competitions(id) ON DELETE RESTRICT,
  version integer NOT NULL,
  status text NOT NULL DEFAULT 'published' CHECK(status IN ('draft','published','superseded','archived')),
  competition_json jsonb NOT NULL,
  scoring_rules_json jsonb NOT NULL DEFAULT '{}',
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  effective_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(competition_id,version)
);

CREATE TABLE competition_challenge_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  competition_snapshot_id uuid NOT NULL REFERENCES competition_snapshots(id) ON DELETE RESTRICT,
  challenge_id uuid NOT NULL REFERENCES challenges(id) ON DELETE RESTRICT,
  challenge_revision_id uuid NOT NULL REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  runtime_revision_id uuid REFERENCES challenge_runtime_revisions(id) ON DELETE RESTRICT,
  score integer NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  opens_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(competition_snapshot_id,challenge_id)
);

ALTER TABLE competitions ADD COLUMN current_snapshot_id uuid;

INSERT INTO competition_snapshots(id,competition_id,version,status,competition_json,scoring_rules_json,effective_at)
SELECT gen_random_uuid(),c.id,1,'published',
  jsonb_build_object('name',c.name,'summary',c.summary,'description',c.description_markdown,'mode',c.mode,'scoringMode',c.scoring_mode,'startsAt',c.starts_at,'endsAt',c.ends_at,'freezeAt',c.freeze_at),
  COALESCE((SELECT config FROM competition_scoring_rules sr WHERE sr.competition_id=c.id),'{}'::jsonb),
  LEAST(now(),c.starts_at)
FROM competitions c WHERE c.status IN ('registration','running','frozen','finished');

UPDATE competitions c SET current_snapshot_id=s.id
FROM competition_snapshots s WHERE s.competition_id=c.id AND s.version=1;

ALTER TABLE competitions ADD CONSTRAINT competitions_current_snapshot_fk
  FOREIGN KEY(current_snapshot_id) REFERENCES competition_snapshots(id) ON DELETE RESTRICT;

INSERT INTO competition_challenge_snapshots(
  id,competition_snapshot_id,challenge_id,challenge_revision_id,runtime_revision_id,score,sort_order,opens_at
)
SELECT gen_random_uuid(),s.id,cc.challenge_id,r.id,rr.id,cc.score,cc.sort_order,cc.opens_at
FROM competition_snapshots s
JOIN competition_challenges cc ON cc.competition_id=s.competition_id
JOIN challenges c ON c.id=cc.challenge_id
JOIN challenge_revisions r ON r.id=c.current_published_revision_id
LEFT JOIN challenge_runtime_revisions rr ON rr.challenge_revision_id=r.id;

ALTER TABLE challenge_instances ADD CONSTRAINT challenge_instances_runtime_revision_fk
  FOREIGN KEY(runtime_revision_id) REFERENCES challenge_runtime_revisions(id) ON DELETE RESTRICT;

ALTER TABLE submissions
  ADD COLUMN challenge_revision_id uuid REFERENCES challenge_revisions(id) ON DELETE RESTRICT,
  ADD COLUMN competition_snapshot_id uuid REFERENCES competition_snapshots(id) ON DELETE RESTRICT;
CREATE INDEX submissions_revision_idx ON submissions(challenge_revision_id,competition_snapshot_id,created_at DESC);

ALTER TABLE writeups ADD COLUMN challenge_revision_id uuid REFERENCES challenge_revisions(id) ON DELETE RESTRICT;

-- +goose Down
-- Immutable history is intentionally retained by expand-and-contract policy.
SELECT 1;
