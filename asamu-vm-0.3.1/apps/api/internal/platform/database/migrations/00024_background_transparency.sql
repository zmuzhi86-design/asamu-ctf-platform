-- +goose Up
-- Keep the platform artwork visible as a subtle page texture without reducing
-- text and card contrast. The frontend renders the white overlay above the
-- image, so these values work for upgraded and newly seeded installs.
UPDATE page_background_configs
SET overlay_color = '#ffffff',
    overlay_opacity = CASE WHEN page_key = 'home' THEN 45 ELSE 55 END,
    asset_opacity = CASE WHEN page_key = 'home' THEN 18 ELSE 12 END,
    blur = 0
WHERE light_asset_key = 'background.platform.light';

-- +goose Down
-- Appearance remains administrator-editable; do not overwrite later changes.
SELECT 1;
