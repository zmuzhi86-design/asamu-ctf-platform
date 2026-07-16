-- +goose Up
-- Rename only untouched product defaults. Custom operator branding is preserved.
UPDATE platform_profiles
SET platform_name = 'asamu',
    short_name = CASE WHEN short_name = 'CM' THEN 'ASAMU' ELSE short_name END,
    homepage_title = CASE
      WHEN homepage_title = '链镜网络安全学习平台' THEN 'asamu 网络安全学习平台'
      ELSE homepage_title
    END,
    updated_at = now()
WHERE platform_name = 'Chain Mirror';

UPDATE challenges
SET author_name = 'asamu Lab', updated_at = now()
WHERE author_name = 'Chain Mirror Lab';

UPDATE challenge_revisions
SET author_name = 'asamu Lab'
WHERE author_name = 'Chain Mirror Lab';

UPDATE competitions
SET slug = 'asamu-practice',
    name = CASE WHEN name = 'Chain Mirror 新生练习赛' THEN 'asamu 新生练习赛' ELSE name END,
    updated_at = now()
WHERE slug = 'chain-mirror-practice'
  AND NOT EXISTS (SELECT 1 FROM competitions WHERE slug = 'asamu-practice');

UPDATE competitions
SET name = 'asamu 新生练习赛', updated_at = now()
WHERE name = 'Chain Mirror 新生练习赛';

-- +goose Down
-- Branding migrations are intentionally irreversible: rolling back application
-- code must not silently overwrite operator-visible names.
SELECT 1;
