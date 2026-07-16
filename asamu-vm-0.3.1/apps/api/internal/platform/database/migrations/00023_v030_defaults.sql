-- +goose Up
-- The 0.3.0 public catalog intentionally starts with seven core directions.
UPDATE challenge_directions
SET status = CASE
      WHEN slug IN ('web','misc','reverse','mobile','pwn','iot','crypto') THEN 'active'
      ELSE 'archived'
    END,
    show_on_home = slug IN ('web','misc','reverse','mobile','pwn','iot','crypto'),
    show_on_library_header = slug IN ('web','misc','reverse','mobile','pwn','iot','crypto'),
    show_on_library_sidebar = slug IN ('web','misc','reverse','mobile','pwn','iot','crypto'),
    updated_at = now();

UPDATE challenge_categories
SET enabled = key IN ('web','misc','reverse','mobile','pwn','iot','crypto');

-- Use the supplied 0.3 background consistently across public and admin pages.
UPDATE page_background_configs
SET light_asset_key = 'background.platform.light',
    dark_asset_key = NULL,
    mobile_asset_key = NULL,
    dark_mobile_asset_key = NULL,
    fit = 'cover',
    position = 'center',
    overlay_color = '#ffffff',
    overlay_opacity = 0,
    asset_opacity = 100,
    blur = 0;

-- +goose Down
-- Direction visibility and appearance are administrator-editable release state.
SELECT 1;
