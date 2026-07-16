-- +goose Up
CREATE TABLE challenge_library_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES platform_profiles(id) ON DELETE CASCADE,
  page_title text NOT NULL DEFAULT '探索题库',
  page_subtitle text NOT NULL DEFAULT '',
  search_placeholder text NOT NULL DEFAULT '搜索题目',
  show_direction_section boolean NOT NULL DEFAULT true,
  show_sidebar boolean NOT NULL DEFAULT true,
  filter_groups_json jsonb NOT NULL DEFAULT '[]',
  default_sort text NOT NULL DEFAULT 'direction',
  page_size integer NOT NULL DEFAULT 20 CHECK(page_size BETWEEN 1 AND 100),
  card_fields_json jsonb NOT NULL DEFAULT '["difficulty","score","solves","tags"]',
  empty_state_json jsonb NOT NULL DEFAULT '{}',
  error_state_json jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(profile_id)
);

-- +goose Down
SELECT 1;
