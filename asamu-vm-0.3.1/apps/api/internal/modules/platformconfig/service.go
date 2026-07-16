package platformconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Profile struct {
	ID                   uuid.UUID       `json:"id"`
	ProfileKey           string          `json:"profileKey"`
	PlatformName         string          `json:"platformName"`
	ShortName            string          `json:"shortName"`
	Slogan               string          `json:"slogan"`
	Description          string          `json:"description"`
	LogoAssetKey         string          `json:"logoAssetKey"`
	FaviconAssetKey      string          `json:"faviconAssetKey"`
	FooterMarkdown       string          `json:"footerMarkdown"`
	ContactJSON          json.RawMessage `json:"contact" gorm:"column:contact_json"`
	DefaultLocale        string          `json:"defaultLocale"`
	Timezone             string          `json:"timezone"`
	HomepageTitle        string          `json:"homepageTitle"`
	DefaultThemeKey      string          `json:"defaultThemeKey"`
	DefaultBackgroundKey string          `json:"defaultBackgroundKey"`
	RuntimeDefaultsJSON  json.RawMessage `json:"runtimeDefaults" gorm:"column:runtime_defaults_json"`
	Version              int             `json:"version"`
}
type NavigationItem struct {
	ID                 uuid.UUID `json:"id"`
	ItemKey            string    `json:"itemKey"`
	Label              string    `json:"label"`
	Href               string    `json:"href"`
	IconAssetKey       string    `json:"iconAssetKey"`
	RequiredFeature    string    `json:"requiredFeature"`
	RequiredPermission string    `json:"requiredPermission"`
	SortOrder          int       `json:"sortOrder"`
	Enabled            bool      `json:"enabled"`
}
type HomepageBlock struct {
	ID         uuid.UUID       `json:"id"`
	BlockKey   string          `json:"blockKey"`
	BlockType  string          `json:"blockType"`
	Title      string          `json:"title"`
	ConfigJSON json.RawMessage `json:"config" gorm:"column:config_json"`
	SortOrder  int             `json:"sortOrder"`
	Enabled    bool            `json:"enabled"`
}
type Direction struct {
	ID                   uuid.UUID `json:"id"`
	Slug                 string    `json:"slug"`
	Name                 string    `json:"name"`
	Subtitle             string    `json:"subtitle"`
	Description          string    `json:"description"`
	IconAssetKey         string    `json:"iconAssetKey"`
	CardAssetKey         string    `json:"cardAssetKey"`
	BannerAssetKey       string    `json:"bannerAssetKey"`
	BackgroundAssetKey   string    `json:"backgroundAssetKey"`
	SortOrder            int       `json:"sortOrder"`
	Status               string    `json:"status"`
	ShowOnHome           bool      `json:"showOnHome"`
	ShowOnLibraryHeader  bool      `json:"showOnLibraryHeader"`
	ShowOnLibrarySidebar bool      `json:"showOnLibrarySidebar"`
	Featured             bool      `json:"featured"`
}
type LibraryConfig struct {
	PageTitle            string          `json:"pageTitle"`
	PageSubtitle         string          `json:"pageSubtitle"`
	SearchPlaceholder    string          `json:"searchPlaceholder"`
	ShowDirectionSection bool            `json:"showDirectionSection"`
	ShowSidebar          bool            `json:"showSidebar"`
	FilterGroupsJSON     json.RawMessage `json:"filterGroups" gorm:"column:filter_groups_json"`
	DefaultSort          string          `json:"defaultSort"`
	PageSize             int             `json:"pageSize"`
	CardFieldsJSON       json.RawMessage `json:"cardFields" gorm:"column:card_fields_json"`
	EmptyStateJSON       json.RawMessage `json:"emptyState" gorm:"column:empty_state_json"`
	ErrorStateJSON       json.RawMessage `json:"errorState" gorm:"column:error_state_json"`
}
type Bootstrap struct {
	Profile          Profile          `json:"profile"`
	Features         map[string]bool  `json:"features"`
	Navigation       []NavigationItem `json:"navigation"`
	HomepageBlocks   []HomepageBlock  `json:"homepageBlocks"`
	Directions       []Direction      `json:"directions"`
	ChallengeLibrary LibraryConfig    `json:"challengeLibrary"`
	PublishedVersion int              `json:"publishedVersion"`
}

func defaults() Bootstrap {
	return Bootstrap{Profile: Profile{PlatformName: "asamu", ShortName: "ASAMU", Slogan: "探索无界 · 攻防未来", DefaultLocale: "zh-CN", Timezone: "Asia/Shanghai", HomepageTitle: "asamu 网络安全学习平台", DefaultThemeKey: "platform-default", ContactJSON: json.RawMessage(`{}`), RuntimeDefaultsJSON: json.RawMessage(`{}`)}, Features: map[string]bool{"registration": true, "teams": true, "writeups": true, "learning": true, "competitions": true, "runtime": false}, Navigation: []NavigationItem{}, HomepageBlocks: []HomepageBlock{}, Directions: []Direction{}, ChallengeLibrary: LibraryConfig{PageTitle: "探索题库", SearchPlaceholder: "搜索题目", ShowDirectionSection: true, ShowSidebar: true, DefaultSort: "direction", PageSize: 20, FilterGroupsJSON: json.RawMessage(`[]`), CardFieldsJSON: json.RawMessage(`["difficulty","score","solves","tags"]`), EmptyStateJSON: json.RawMessage(`{}`), ErrorStateJSON: json.RawMessage(`{}`)}}
}

func (s *Service) Bootstrap(ctx context.Context) (Bootstrap, error) {
	var version struct {
		Version      int
		SnapshotJSON json.RawMessage
	}
	err := s.db.WithContext(ctx).Table("platform_setting_versions").Select("version,snapshot_json").Where("status='published'").Order("published_at DESC,created_at DESC").Take(&version).Error
	if err == nil {
		var result Bootstrap
		if json.Unmarshal(version.SnapshotJSON, &result) == nil {
			result.PublishedVersion = version.Version
			return result, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Bootstrap{}, err
	}
	return s.load(ctx, true)
}

func (s *Service) Draft(ctx context.Context) (Bootstrap, error) {
	return s.load(ctx, false)
}

func (s *Service) load(ctx context.Context, publicOnly bool) (Bootstrap, error) {
	result := defaults()
	var profile Profile
	if err := s.db.WithContext(ctx).Table("platform_profiles").Order("updated_at DESC").Take(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.db.WithContext(ctx).Table("challenge_directions").Where("status='active'").Order("sort_order,name").Find(&result.Directions).Error; err != nil {
				return Bootstrap{}, err
			}
			return result, nil
		}
		return Bootstrap{}, err
	}
	result.Profile = profile
	var flags []struct {
		FeatureKey string
		Enabled    bool
	}
	if err := s.db.WithContext(ctx).Table("platform_feature_flags").Where("profile_id=?", profile.ID).Find(&flags).Error; err != nil {
		return Bootstrap{}, err
	}
	result.Features = map[string]bool{}
	for _, flag := range flags {
		result.Features[flag.FeatureKey] = flag.Enabled
	}
	navigation := s.db.WithContext(ctx).Table("navigation_items").Where("profile_id=?", profile.ID)
	homepage := s.db.WithContext(ctx).Table("homepage_blocks").Where("profile_id=?", profile.ID)
	directions := s.db.WithContext(ctx).Table("challenge_directions")
	if publicOnly {
		navigation = navigation.Where("enabled=true")
		homepage = homepage.Where("enabled=true")
		directions = directions.Where("status='active'")
	}
	if err := navigation.Order("sort_order").Find(&result.Navigation).Error; err != nil {
		return Bootstrap{}, err
	}
	if err := homepage.Order("sort_order").Find(&result.HomepageBlocks).Error; err != nil {
		return Bootstrap{}, err
	}
	if err := s.db.WithContext(ctx).Table("challenge_library_configs").Where("profile_id=?", profile.ID).Take(&result.ChallengeLibrary).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Bootstrap{}, err
	}
	if err := directions.Order("sort_order,name").Find(&result.Directions).Error; err != nil {
		return Bootstrap{}, err
	}
	return result, nil
}

func (s *Service) SaveProfile(ctx context.Context, input Profile) (Profile, error) {
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	if input.ProfileKey == "" {
		input.ProfileKey = "default"
	}
	if input.PlatformName == "" {
		input.PlatformName = "asamu"
	}
	if len(input.ContactJSON) == 0 {
		input.ContactJSON = json.RawMessage(`{}`)
	}
	if len(input.RuntimeDefaultsJSON) == 0 {
		input.RuntimeDefaultsJSON = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	values := map[string]any{"id": input.ID, "profile_key": input.ProfileKey, "platform_name": input.PlatformName, "short_name": input.ShortName, "slogan": input.Slogan, "description": input.Description, "logo_asset_key": input.LogoAssetKey, "favicon_asset_key": input.FaviconAssetKey, "footer_markdown": input.FooterMarkdown, "contact_json": input.ContactJSON, "default_locale": input.DefaultLocale, "timezone": input.Timezone, "homepage_title": input.HomepageTitle, "default_theme_key": input.DefaultThemeKey, "default_background_key": input.DefaultBackgroundKey, "runtime_defaults_json": input.RuntimeDefaultsJSON, "updated_at": now}
	if err := s.db.WithContext(ctx).Table("platform_profiles").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "profile_key"}}, DoUpdates: clause.AssignmentColumns([]string{"platform_name", "short_name", "slogan", "description", "logo_asset_key", "favicon_asset_key", "footer_markdown", "contact_json", "default_locale", "timezone", "homepage_title", "default_theme_key", "default_background_key", "runtime_defaults_json", "updated_at"})}).Create(values).Error; err != nil {
		return Profile{}, err
	}
	return s.profile(ctx, input.ProfileKey)
}

func (s *Service) SaveDraft(ctx context.Context, input Bootstrap) (Bootstrap, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		service := New(tx)
		profile, err := service.SaveProfile(ctx, input.Profile)
		if err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM platform_feature_flags WHERE profile_id = ?", profile.ID).Error; err != nil {
			return err
		}
		for key, enabled := range input.Features {
			if err := tx.Table("platform_feature_flags").Create(map[string]any{"profile_id": profile.ID, "feature_key": key, "enabled": enabled, "config_json": json.RawMessage(`{}`), "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM navigation_items WHERE profile_id = ?", profile.ID).Error; err != nil {
			return err
		}
		for _, item := range input.Navigation {
			if item.ID == uuid.Nil {
				item.ID = uuid.New()
			}
			if err := tx.Table("navigation_items").Create(map[string]any{"id": item.ID, "profile_id": profile.ID, "item_key": item.ItemKey, "label": item.Label, "href": item.Href, "icon_asset_key": item.IconAssetKey, "required_feature": item.RequiredFeature, "required_permission": item.RequiredPermission, "sort_order": item.SortOrder, "enabled": item.Enabled}).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM homepage_blocks WHERE profile_id = ?", profile.ID).Error; err != nil {
			return err
		}
		for _, block := range input.HomepageBlocks {
			if block.ID == uuid.Nil {
				block.ID = uuid.New()
			}
			if len(block.ConfigJSON) == 0 {
				block.ConfigJSON = json.RawMessage(`{}`)
			}
			if err := tx.Table("homepage_blocks").Create(map[string]any{"id": block.ID, "profile_id": profile.ID, "block_key": block.BlockKey, "block_type": block.BlockType, "title": block.Title, "config_json": block.ConfigJSON, "sort_order": block.SortOrder, "enabled": block.Enabled}).Error; err != nil {
				return err
			}
		}
		library := input.ChallengeLibrary
		if len(library.FilterGroupsJSON) == 0 {
			library.FilterGroupsJSON = json.RawMessage(`[]`)
		}
		if len(library.CardFieldsJSON) == 0 {
			library.CardFieldsJSON = json.RawMessage(`[]`)
		}
		if len(library.EmptyStateJSON) == 0 {
			library.EmptyStateJSON = json.RawMessage(`{}`)
		}
		if len(library.ErrorStateJSON) == 0 {
			library.ErrorStateJSON = json.RawMessage(`{}`)
		}
		if library.PageSize < 1 {
			library.PageSize = 20
		} else if library.PageSize > 100 {
			return httpx.NewError(http.StatusBadRequest, "INVALID_LIBRARY_PAGE_SIZE", "题库每页数量必须在 1 到 100 之间")
		}
		values := map[string]any{"id": uuid.New(), "profile_id": profile.ID, "page_title": library.PageTitle, "page_subtitle": library.PageSubtitle, "search_placeholder": library.SearchPlaceholder, "show_direction_section": library.ShowDirectionSection, "show_sidebar": library.ShowSidebar, "filter_groups_json": library.FilterGroupsJSON, "default_sort": library.DefaultSort, "page_size": library.PageSize, "card_fields_json": library.CardFieldsJSON, "empty_state_json": library.EmptyStateJSON, "error_state_json": library.ErrorStateJSON, "updated_at": time.Now().UTC()}
		return tx.Table("challenge_library_configs").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "profile_id"}}, DoUpdates: clause.AssignmentColumns([]string{"page_title", "page_subtitle", "search_placeholder", "show_direction_section", "show_sidebar", "filter_groups_json", "default_sort", "page_size", "card_fields_json", "empty_state_json", "error_state_json", "updated_at"})}).Create(values).Error
	})
	if err != nil {
		return Bootstrap{}, err
	}
	return s.Draft(ctx)
}

func (s *Service) profile(ctx context.Context, key string) (Profile, error) {
	var out Profile
	err := s.db.WithContext(ctx).Table("platform_profiles").Where("profile_key=?", key).Take(&out).Error
	return out, err
}

func (s *Service) Publish(ctx context.Context, actor uuid.UUID) (Bootstrap, error) {
	var published Bootstrap
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current := New(tx)
		value, err := current.load(ctx, true)
		if err != nil {
			return err
		}
		var version int
		if err := tx.Table("platform_setting_versions").Where("profile_id=?", value.Profile.ID).Select("COALESCE(MAX(version),0)+1").Scan(&version).Error; err != nil {
			return err
		}
		value.PublishedVersion = version
		snapshot, _ := json.Marshal(value)
		now := time.Now().UTC()
		if err := tx.Table("platform_setting_versions").Where("profile_id=? AND status='published'", value.Profile.ID).Update("status", "archived").Error; err != nil {
			return err
		}
		if err := tx.Table("platform_setting_versions").Create(map[string]any{"id": uuid.New(), "profile_id": value.Profile.ID, "version": version, "snapshot_json": snapshot, "status": "published", "created_by": actor, "created_at": now, "published_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Table("platform_profiles").Where("id=?", value.Profile.ID).Updates(map[string]any{"status": "published", "version": version, "published_at": now}).Error; err != nil {
			return err
		}
		published = value
		return nil
	})
	return published, err
}

func (s *Service) Directions(ctx context.Context, admin bool) ([]Direction, error) {
	var rows []Direction
	q := s.db.WithContext(ctx).Table("challenge_directions")
	if !admin {
		q = q.Where("status='active'")
	}
	err := q.Order("sort_order,name").Find(&rows).Error
	return rows, err
}
func (s *Service) SaveDirection(ctx context.Context, value Direction) (Direction, error) {
	if value.Status == "" {
		value.Status = "active"
	}
	now := time.Now().UTC()
	updates := map[string]any{"slug": value.Slug, "name": value.Name, "subtitle": value.Subtitle, "description": value.Description, "icon_asset_key": value.IconAssetKey, "card_asset_key": value.CardAssetKey, "banner_asset_key": value.BannerAssetKey, "background_asset_key": value.BackgroundAssetKey, "sort_order": value.SortOrder, "status": value.Status, "show_on_home": value.ShowOnHome, "show_on_library_header": value.ShowOnLibraryHeader, "show_on_library_sidebar": value.ShowOnLibrarySidebar, "featured": value.Featured, "updated_at": now}
	if value.ID != uuid.Nil {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Table("challenge_directions").Where("id=?", value.ID).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			return syncChallengeCategory(tx, value)
		})
		if err != nil {
			return Direction{}, err
		}
		var out Direction
		err = s.db.WithContext(ctx).Table("challenge_directions").Where("id=?", value.ID).Take(&out).Error
		return out, err
	}
	value.ID = uuid.New()
	updates["id"] = value.ID
	updates["created_at"] = now
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("challenge_directions").Create(updates).Error; err != nil {
			return err
		}
		return syncChallengeCategory(tx, value)
	})
	if err != nil {
		return Direction{}, err
	}
	var out Direction
	err = s.db.WithContext(ctx).Table("challenge_directions").Where("slug=?", value.Slug).Take(&out).Error
	return out, err
}
func syncChallengeCategory(tx *gorm.DB, value Direction) error {
	category := map[string]any{"id": value.ID, "key": value.Slug, "name": value.Name, "description": value.Description, "scene_asset_key": value.CardAssetKey, "sort_order": value.SortOrder, "enabled": value.Status == "active"}
	return tx.Table("challenge_categories").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"key", "name", "description", "scene_asset_key", "sort_order", "enabled"})}).Create(category).Error
}
func (s *Service) ArchiveDirection(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("challenge_directions").Where("id=?", id).Updates(map[string]any{"status": "archived", "show_on_home": false, "show_on_library_header": false, "show_on_library_sidebar": false, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Table("challenge_categories").Where("id=?", id).Update("enabled", false).Error; err != nil {
			return err
		}
		return tx.Table("learning_paths").Where("direction_id=?", id).Updates(map[string]any{"status": "archived", "featured": false, "updated_at": time.Now().UTC()}).Error
	})
}
