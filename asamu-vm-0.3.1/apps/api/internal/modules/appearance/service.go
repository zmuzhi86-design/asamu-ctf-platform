package appearance

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

type Background struct {
	ID                 uuid.UUID      `json:"id"`
	PageKey            string         `json:"pageKey"`
	ScopeType          string         `json:"scopeType"`
	LightAssetKey      string         `json:"lightAssetKey"`
	DarkAssetKey       string         `json:"darkAssetKey,omitempty"`
	MobileAssetKey     string         `json:"mobileAssetKey,omitempty"`
	DarkMobileAssetKey string         `json:"darkMobileAssetKey,omitempty"`
	Fit                string         `json:"fit"`
	Position           string         `json:"position"`
	OverlayColor       string         `json:"overlayColor"`
	Status             string         `json:"status"`
	ScopeID            *uuid.UUID     `json:"scopeId,omitempty"`
	FocalPoint         map[string]int `json:"focalPoint"`
	OverlayOpacity     int            `json:"overlayOpacity"`
	AssetOpacity       int            `json:"assetOpacity"`
	Blur               int            `json:"blur"`
	Version            int            `json:"version"`
	StartsAt           *time.Time     `json:"startsAt,omitempty"`
	EndsAt             *time.Time     `json:"endsAt,omitempty"`
}
type Theme struct {
	ID             uuid.UUID         `json:"id"`
	ThemeKey       string            `json:"themeKey"`
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	CurrentVersion int               `json:"currentVersion"`
	Tokens         map[string]string `json:"tokens"`
}

var allowedTokenKeys = map[string]bool{"color.primary": true, "color.primarySoft": true, "color.accent": true, "color.canvas": true, "color.card": true, "color.ink": true, "color.muted": true, "color.line": true, "radius.card": true, "shadow.pixel": true, "font.display": true, "font.body": true}

func (s *Service) CurrentTheme(ctx context.Context, scopeType string, scopeID *uuid.UUID) (Theme, error) {
	now := time.Now().UTC()
	var row struct {
		ID                     uuid.UUID
		ThemeKey, Name, Status string
		CurrentVersion         int
		Tokens                 []byte
	}
	query := s.db.WithContext(ctx).Table("theme_bindings tb").Select("t.id,t.theme_key,t.name,t.status,t.current_version,tv.tokens").Joins("JOIN theme_versions tv ON tv.id=tb.theme_version_id").Joins("JOIN themes t ON t.id=tv.theme_id").Where("tb.status='published' AND (tb.starts_at IS NULL OR tb.starts_at<=?) AND (tb.ends_at IS NULL OR tb.ends_at>?)", now, now)
	if scopeID != nil {
		query = query.Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN tb.scope_type=? AND tb.scope_id=? THEN 0 WHEN tb.scope_type='platform' THEN 1 ELSE 2 END", Vars: []any{scopeType, *scopeID}}})
	} else {
		query = query.Where("tb.scope_type='platform'").Order("tb.priority DESC")
	}
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Theme{ThemeKey: "asamu-default", Name: "asamu 默认主题", Status: "published", CurrentVersion: 1, Tokens: map[string]string{}}, nil
		}
		return Theme{}, err
	}
	theme := Theme{ID: row.ID, ThemeKey: row.ThemeKey, Name: row.Name, Status: row.Status, CurrentVersion: row.CurrentVersion, Tokens: map[string]string{}}
	_ = json.Unmarshal(row.Tokens, &theme.Tokens)
	return theme, nil
}
func (s *Service) CurrentBackgrounds(ctx context.Context, scopeType string, scopeID *uuid.UUID) ([]Background, error) {
	now := time.Now().UTC()
	var rows []struct {
		ID                                                                                                                       uuid.UUID
		PageKey, ScopeType, LightAssetKey, DarkAssetKey, MobileAssetKey, DarkMobileAssetKey, Fit, Position, OverlayColor, Status string
		ScopeID                                                                                                                  *uuid.UUID
		FocalPoint                                                                                                               []byte
		OverlayOpacity, AssetOpacity, Blur, Version                                                                              int
		StartsAt, EndsAt                                                                                                         *time.Time
	}
	query := s.db.WithContext(ctx).Table("page_background_configs").Where("status='published' AND (starts_at IS NULL OR starts_at<=?) AND (ends_at IS NULL OR ends_at>?)", now, now)
	if scopeID != nil {
		query = query.Where("(scope_type=? AND scope_id=?) OR scope_type='platform'", scopeType, *scopeID).Order("page_key").Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN scope_type=? AND scope_id=? THEN 0 ELSE 1 END", Vars: []any{scopeType, *scopeID}}})
	} else {
		query = query.Where("scope_type='platform'").Order("page_key,version DESC")
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := []Background{}
	for _, row := range rows {
		if seen[row.PageKey] {
			continue
		}
		seen[row.PageKey] = true
		item := Background{ID: row.ID, PageKey: row.PageKey, ScopeType: row.ScopeType, ScopeID: row.ScopeID, LightAssetKey: row.LightAssetKey, DarkAssetKey: row.DarkAssetKey, MobileAssetKey: row.MobileAssetKey, DarkMobileAssetKey: row.DarkMobileAssetKey, Fit: row.Fit, Position: row.Position, OverlayColor: row.OverlayColor, OverlayOpacity: row.OverlayOpacity, AssetOpacity: row.AssetOpacity, Blur: row.Blur, Status: row.Status, Version: row.Version, FocalPoint: map[string]int{}, StartsAt: row.StartsAt, EndsAt: row.EndsAt}
		_ = json.Unmarshal(row.FocalPoint, &item.FocalPoint)
		items = append(items, item)
	}
	return items, nil
}
func (s *Service) ListBackgrounds(ctx context.Context) ([]Background, error) {
	var rows []struct {
		ID                                                                                                                       uuid.UUID
		PageKey, ScopeType, LightAssetKey, DarkAssetKey, MobileAssetKey, DarkMobileAssetKey, Fit, Position, OverlayColor, Status string
		ScopeID                                                                                                                  *uuid.UUID
		FocalPoint                                                                                                               []byte
		OverlayOpacity, AssetOpacity, Blur, Version                                                                              int
		StartsAt, EndsAt                                                                                                         *time.Time
	}
	if err := s.db.WithContext(ctx).Table("page_background_configs").Order("page_key,version DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]Background, 0, len(rows))
	for _, row := range rows {
		item := Background{ID: row.ID, PageKey: row.PageKey, ScopeType: row.ScopeType, ScopeID: row.ScopeID, LightAssetKey: row.LightAssetKey, DarkAssetKey: row.DarkAssetKey, MobileAssetKey: row.MobileAssetKey, DarkMobileAssetKey: row.DarkMobileAssetKey, Fit: row.Fit, Position: row.Position, OverlayColor: row.OverlayColor, OverlayOpacity: row.OverlayOpacity, AssetOpacity: row.AssetOpacity, Blur: row.Blur, Status: row.Status, Version: row.Version, FocalPoint: map[string]int{}, StartsAt: row.StartsAt, EndsAt: row.EndsAt}
		_ = json.Unmarshal(row.FocalPoint, &item.FocalPoint)
		items = append(items, item)
	}
	return items, nil
}
func (s *Service) SaveBackground(ctx context.Context, input Background) (Background, error) {
	if !validFit(input.Fit) {
		return Background{}, httpx.NewError(http.StatusBadRequest, "INVALID_BACKGROUND_FIT", "背景显示模式不合法")
	}
	if input.OverlayOpacity < 0 || input.OverlayOpacity > 100 || input.AssetOpacity < 0 || input.AssetOpacity > 100 || input.Blur < 0 || input.Blur > 30 {
		return Background{}, httpx.NewError(http.StatusBadRequest, "INVALID_BACKGROUND_STYLE", "背景透明度或模糊值超出范围")
	}
	focal, _ := json.Marshal(input.FocalPoint)
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
		input.Status = "draft"
		input.Version = 1
		record := map[string]any{"id": input.ID, "page_key": input.PageKey, "scope_type": input.ScopeType, "scope_id": input.ScopeID, "light_asset_key": input.LightAssetKey, "dark_asset_key": nullString(input.DarkAssetKey), "mobile_asset_key": nullString(input.MobileAssetKey), "dark_mobile_asset_key": nullString(input.DarkMobileAssetKey), "fit": input.Fit, "position": input.Position, "focal_point": focal, "overlay_color": input.OverlayColor, "overlay_opacity": input.OverlayOpacity, "asset_opacity": input.AssetOpacity, "blur": input.Blur, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "status": "draft", "version": 1, "created_at": time.Now().UTC()}
		return input, s.db.WithContext(ctx).Table("page_background_configs").Create(record).Error
	}
	var current struct {
		ID        uuid.UUID
		PageKey   string
		ScopeType string
		ScopeID   *uuid.UUID
		Status    string
		Version   int
	}
	if err := s.db.WithContext(ctx).Table("page_background_configs").Where("id=?", input.ID).Take(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Background{}, httpx.NewError(http.StatusNotFound, "BACKGROUND_NOT_FOUND", "背景配置不存在")
		}
		return Background{}, err
	}
	targetID, targetVersion := current.ID, current.Version
	if current.Status != "draft" {
		var draft struct {
			ID      uuid.UUID
			Version int
		}
		err := s.db.WithContext(ctx).Table("page_background_configs").Select("id,version").Where("page_key=? AND scope_type=? AND scope_id IS NOT DISTINCT FROM ? AND status='draft'", current.PageKey, current.ScopeType, current.ScopeID).Order("version DESC").Take(&draft).Error
		if err == nil {
			targetID, targetVersion = draft.ID, draft.Version
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Background{}, err
		} else {
			targetID, targetVersion = uuid.New(), current.Version+1
			record := map[string]any{"id": targetID, "page_key": current.PageKey, "scope_type": current.ScopeType, "scope_id": current.ScopeID, "light_asset_key": input.LightAssetKey, "dark_asset_key": nullString(input.DarkAssetKey), "mobile_asset_key": nullString(input.MobileAssetKey), "dark_mobile_asset_key": nullString(input.DarkMobileAssetKey), "fit": input.Fit, "position": input.Position, "focal_point": focal, "overlay_color": input.OverlayColor, "overlay_opacity": input.OverlayOpacity, "asset_opacity": input.AssetOpacity, "blur": input.Blur, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "status": "draft", "version": targetVersion, "created_at": time.Now().UTC()}
			if err := s.db.WithContext(ctx).Table("page_background_configs").Create(record).Error; err != nil {
				return Background{}, err
			}
			input.ID, input.PageKey, input.ScopeType, input.ScopeID, input.Status, input.Version = targetID, current.PageKey, current.ScopeType, current.ScopeID, "draft", targetVersion
			return input, nil
		}
	}
	updates := map[string]any{"light_asset_key": input.LightAssetKey, "dark_asset_key": nullString(input.DarkAssetKey), "mobile_asset_key": nullString(input.MobileAssetKey), "dark_mobile_asset_key": nullString(input.DarkMobileAssetKey), "fit": input.Fit, "position": input.Position, "focal_point": focal, "overlay_color": input.OverlayColor, "overlay_opacity": input.OverlayOpacity, "asset_opacity": input.AssetOpacity, "blur": input.Blur, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "status": "draft"}
	if err := s.db.WithContext(ctx).Table("page_background_configs").Where("id=?", targetID).Updates(updates).Error; err != nil {
		return Background{}, err
	}
	input.ID, input.PageKey, input.ScopeType, input.ScopeID, input.Status, input.Version = targetID, current.PageKey, current.ScopeType, current.ScopeID, "draft", targetVersion
	return input, nil
}
func (s *Service) PublishBackground(ctx context.Context, id, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var background Background
		if err := tx.Table("page_background_configs").Where("id=?", id).First(&background).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "BACKGROUND_NOT_FOUND", "背景配置不存在")
		}
		if err := tx.Table("page_background_configs").Where("page_key=? AND scope_type=? AND scope_id IS NOT DISTINCT FROM ? AND id<>?", background.PageKey, background.ScopeType, background.ScopeID, id).Update("status", "archived").Error; err != nil {
			return err
		}
		if err := tx.Table("page_background_configs").Where("id=?", id).Update("status", "published").Error; err != nil {
			return err
		}
		return tx.Table("audit_logs").Create(map[string]any{"id": uuid.New(), "actor_id": actor, "actor_type": "user", "action": "background.publish", "resource_type": "page_background", "resource_id": id.String(), "created_at": time.Now().UTC()}).Error
	})
}
func (s *Service) RollbackBackground(ctx context.Context, id uuid.UUID) (Background, error) {
	var current Background
	if err := s.db.WithContext(ctx).Table("page_background_configs").Where("id=?", id).First(&current).Error; err != nil {
		return Background{}, httpx.NewError(http.StatusNotFound, "BACKGROUND_NOT_FOUND", "背景配置不存在")
	}
	var previous Background
	if err := s.db.WithContext(ctx).Table("page_background_configs").Where("page_key=? AND scope_type=? AND scope_id IS NOT DISTINCT FROM ? AND version<?", current.PageKey, current.ScopeType, current.ScopeID, current.Version).Order("version DESC").First(&previous).Error; err != nil {
		return Background{}, httpx.NewError(http.StatusConflict, "NO_PREVIOUS_VERSION", "没有可回滚的背景版本")
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("page_background_configs").Where("id=?", current.ID).Update("status", "archived").Error; err != nil {
			return err
		}
		return tx.Table("page_background_configs").Where("id=?", previous.ID).Update("status", "published").Error
	}); err != nil {
		return Background{}, err
	}
	previous.Status = "published"
	return previous, nil
}
func (s *Service) SaveTheme(ctx context.Context, actor uuid.UUID, key, name string, tokens map[string]string) (Theme, error) {
	for token := range tokens {
		if !allowedTokenKeys[token] {
			return Theme{}, httpx.NewError(http.StatusBadRequest, "UNSUPPORTED_THEME_TOKEN", "主题包含不受支持的 Token："+token)
		}
	}
	payload, _ := json.Marshal(tokens)
	theme := Theme{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			ID             uuid.UUID
			CurrentVersion int
		}
		err := tx.Table("themes").Where("theme_key=?", key).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row.ID = uuid.New()
			row.CurrentVersion = 0
			if err := tx.Table("themes").Create(map[string]any{"id": row.ID, "theme_key": key, "name": name, "status": "draft", "current_version": 1, "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		next := row.CurrentVersion + 1
		if row.CurrentVersion == 0 {
			next = 1
		}
		versionID := uuid.New()
		if err := tx.Table("theme_versions").Create(map[string]any{"id": versionID, "theme_id": row.ID, "version": next, "tokens": payload, "created_by": actor, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := tx.Table("themes").Where("id=?", row.ID).Updates(map[string]any{"name": name, "status": "draft", "current_version": next}).Error; err != nil {
			return err
		}
		theme = Theme{ID: row.ID, ThemeKey: key, Name: name, Status: "draft", CurrentVersion: next, Tokens: tokens}
		return nil
	})
	return theme, err
}
func validFit(value string) bool {
	for _, fit := range []string{"cover", "contain", "repeat", "repeat-x", "centered", "fixed"} {
		if value == fit {
			return true
		}
	}
	return false
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ = clause.Locking{}
