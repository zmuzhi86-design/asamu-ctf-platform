package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/storage"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxFileSize         int64 = 25 << 20
	maxTeamAvatarSize   int64 = 5 << 20
	maxPixels           int64 = 40_000_000
	maxTeamAvatarPixels int64 = 4096 * 4096
)

type Service struct {
	db            *gorm.DB
	storage       storage.Storage
	publicBaseURL string
}

func New(db *gorm.DB, store storage.Storage, publicBaseURL string) *Service {
	return &Service{db: db, storage: store, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

type Record struct {
	ID               uuid.UUID      `json:"id"`
	AssetKey         string         `json:"assetKey"`
	Name             string         `json:"name"`
	Category         string         `json:"category"`
	AltText          string         `json:"altText"`
	Status           string         `json:"status"`
	Fit              string         `json:"fit"`
	Position         string         `json:"position"`
	Tags             []string       `json:"tags"`
	CurrentVersion   int            `json:"currentVersion"`
	Versions         []Version      `json:"versions"`
	FocalPoint       map[string]int `json:"focalPoint"`
	SafeArea         map[string]int `json:"safeArea"`
	ApplicablePages  []string       `json:"applicablePages"`
	FallbackAssetKey string         `json:"fallbackAssetKey,omitempty"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}
type Version struct {
	ID            uuid.UUID `json:"id"`
	Version       int       `json:"version"`
	URL           string    `json:"url,omitempty"`
	MobileURL     string    `json:"mobileUrl,omitempty"`
	DarkURL       string    `json:"darkUrl,omitempty"`
	DarkMobileURL string    `json:"darkMobileUrl,omitempty"`
	MIMEType      string    `json:"mimeType,omitempty"`
	SHA256        string    `json:"sha256,omitempty"`
	Note          string    `json:"note,omitempty"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	HasAlpha      bool      `json:"hasAlpha"`
	RiskFlags     []string  `json:"riskFlags"`
	CreatedAt     time.Time `json:"createdAt"`
}
type CreateInput struct {
	AssetKey, Name, Category, AltText, Fit, Position, FallbackAssetKey string
	Tags, ApplicablePages                                              []string
	FocalPoint, SafeArea                                               map[string]int
}
type Upload struct {
	Name        string
	ContentType string
	Data        []byte
}

func (s *Service) UpsertTeamAvatar(ctx context.Context, actor, teamID uuid.UUID, upload Upload) (string, error) {
	metadata, err := inspectTeamAvatar(upload)
	if err != nil {
		return "", err
	}
	assetKey := "team.custom." + strings.ReplaceAll(teamID.String(), "-", "") + ".avatar"
	objectKey := fmt.Sprintf("team-assets/%s/%s%s", teamID.String(), uuid.NewString(), metadata.Extension)
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(upload.Data), int64(len(upload.Data)), metadata.MIME); err != nil {
		return "", err
	}
	digest := sha256.Sum256(upload.Data)
	hash := hex.EncodeToString(digest[:])
	riskJSON, _ := json.Marshal(metadata.Risks)
	pages, _ := json.Marshal([]string{"team_list", "team_detail"})
	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "team-avatar:"+teamID.String()).Error; err != nil {
			return err
		}
		var current struct {
			ID             uuid.UUID
			CurrentVersion int
		}
		query := tx.Table("assets").Select("id,current_version").Where("asset_key=?", assetKey).Take(&current)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			assetID := uuid.New()
			var categoryID *uuid.UUID
			var category struct{ ID uuid.UUID }
			categoryQuery := tx.Table("asset_categories").Select("id").Where("key='team'").Take(&category)
			if categoryQuery.Error == nil {
				categoryID = &category.ID
			} else if !errors.Is(categoryQuery.Error, gorm.ErrRecordNotFound) {
				return categoryQuery.Error
			}
			if err := tx.Table("assets").Create(map[string]any{"id": assetID, "asset_key": assetKey, "name": "战队自定义头像", "category_id": categoryID, "alt_text": "战队自定义头像", "status": "published", "fit": "cover", "position": "center", "focal_point": []byte(`{"x":50,"y":50}`), "safe_area": []byte(`{"top":0,"right":0,"bottom":0,"left":0}`), "applicable_pages": pages, "fallback_asset_key": "team.honor.verified", "current_version": 1, "created_by": actor, "created_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			return tx.Table("asset_versions").Create(map[string]any{"id": uuid.New(), "asset_id": assetID, "version": 1, "object_key": objectKey, "mime_type": metadata.MIME, "width": metadata.Width, "height": metadata.Height, "has_alpha": metadata.HasAlpha, "sha256": hash, "risk_flags": riskJSON, "note": "队长上传", "created_by": actor, "created_at": now}).Error
		}
		if query.Error != nil {
			return query.Error
		}
		var next int
		if err := tx.Table("asset_versions").Where("asset_id=?", current.ID).Select("COALESCE(MAX(version),0)+1").Scan(&next).Error; err != nil {
			return err
		}
		if err := tx.Table("asset_versions").Create(map[string]any{"id": uuid.New(), "asset_id": current.ID, "version": next, "object_key": objectKey, "mime_type": metadata.MIME, "width": metadata.Width, "height": metadata.Height, "has_alpha": metadata.HasAlpha, "sha256": hash, "risk_flags": riskJSON, "note": "队长更新", "created_by": actor, "created_at": now}).Error; err != nil {
			return err
		}
		return tx.Table("assets").Where("id=?", current.ID).Updates(map[string]any{"current_version": next, "status": "published", "updated_at": now}).Error
	})
	if err != nil {
		_ = s.storage.Delete(context.Background(), objectKey)
		return "", err
	}
	return assetKey, nil
}

type Slot struct {
	ID                 uuid.UUID `json:"id"`
	SlotKey            string    `json:"slotKey"`
	Name               string    `json:"name"`
	PageKey            string    `json:"page"`
	Fit                string    `json:"fit"`
	Position           string    `json:"position"`
	Enabled            bool      `json:"enabled"`
	Version            int       `json:"version"`
	Bindings           []Binding `json:"bindings" gorm:"-"`
	AssetKey           string    `json:"assetKey" gorm:"-"`
	MobileAssetKey     string    `json:"mobileAssetKey,omitempty" gorm:"-"`
	DarkAssetKey       string    `json:"darkAssetKey,omitempty" gorm:"-"`
	DarkMobileAssetKey string    `json:"darkMobileAssetKey,omitempty" gorm:"-"`
}
type Binding struct {
	ID                    uuid.UUID  `json:"id"`
	ScopeType             string     `json:"scopeType"`
	ScopeID               *uuid.UUID `json:"scopeId,omitempty"`
	LightDesktopVersionID *uuid.UUID `json:"lightDesktopVersionId,omitempty"`
	LightMobileVersionID  *uuid.UUID `json:"lightMobileVersionId,omitempty"`
	DarkDesktopVersionID  *uuid.UUID `json:"darkDesktopVersionId,omitempty"`
	DarkMobileVersionID   *uuid.UUID `json:"darkMobileVersionId,omitempty"`
	StartsAt              *time.Time `json:"startsAt,omitempty"`
	EndsAt                *time.Time `json:"endsAt,omitempty"`
	Priority              int        `json:"priority"`
	Status                string     `json:"status"`
}

func (s *Service) List(ctx context.Context, search, category, status string, page, size int) (httpx.Page[Record], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("assets a").Joins("LEFT JOIN asset_categories ac ON ac.id=a.category_id")
	if search != "" {
		query = query.Where("a.asset_key ILIKE ? OR a.name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if category != "" {
		query = query.Where("ac.key=?", category)
	}
	if status != "" {
		query = query.Where("a.status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[Record]{}, err
	}
	var basics []struct {
		ID                                                                         uuid.UUID
		AssetKey, Name, Category, AltText, Status, Fit, Position, FallbackAssetKey string
		CurrentVersion                                                             int
		FocalPoint, SafeArea, ApplicablePages                                      []byte
		UpdatedAt                                                                  time.Time
	}
	if err := query.Select("a.id,a.asset_key,a.name,coalesce(ac.key,'') AS category,a.alt_text,a.status,a.fit,a.position,a.fallback_asset_key,a.current_version,a.focal_point,a.safe_area,a.applicable_pages,a.updated_at").Order("a.updated_at DESC").Offset((page - 1) * size).Limit(size).Scan(&basics).Error; err != nil {
		return httpx.Page[Record]{}, err
	}
	items := make([]Record, 0, len(basics))
	for _, row := range basics {
		record := Record{ID: row.ID, AssetKey: row.AssetKey, Name: row.Name, Category: row.Category, AltText: row.AltText, Status: row.Status, Fit: row.Fit, Position: row.Position, FallbackAssetKey: row.FallbackAssetKey, CurrentVersion: row.CurrentVersion, UpdatedAt: row.UpdatedAt, FocalPoint: map[string]int{}, SafeArea: map[string]int{}, ApplicablePages: []string{}, Tags: []string{}, Versions: []Version{}}
		_ = json.Unmarshal(row.FocalPoint, &record.FocalPoint)
		_ = json.Unmarshal(row.SafeArea, &record.SafeArea)
		_ = json.Unmarshal(row.ApplicablePages, &record.ApplicablePages)
		record.Versions, _ = s.versions(ctx, row.ID)
		_ = s.db.WithContext(ctx).Table("asset_tag_links atl").Joins("JOIN asset_tags at ON at.id=atl.tag_id").Where("atl.asset_id=?", row.ID).Pluck("at.name", &record.Tags).Error
		items = append(items, record)
	}
	return httpx.Page[Record]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Get(ctx context.Context, identifier string) (Record, error) {
	var row struct {
		ID                                                                         uuid.UUID
		AssetKey, Name, Category, AltText, Status, Fit, Position, FallbackAssetKey string
		CurrentVersion                                                             int
		FocalPoint, SafeArea, ApplicablePages                                      []byte
		UpdatedAt                                                                  time.Time
	}
	query := s.db.WithContext(ctx).Table("assets a").Select("a.id,a.asset_key,a.name,coalesce(ac.key,'') AS category,a.alt_text,a.status,a.fit,a.position,a.fallback_asset_key,a.current_version,a.focal_point,a.safe_area,a.applicable_pages,a.updated_at").Joins("LEFT JOIN asset_categories ac ON ac.id=a.category_id")
	if parsed, err := uuid.Parse(identifier); err == nil {
		query = query.Where("a.id=?", parsed)
	} else {
		query = query.Where("a.asset_key=?", identifier)
	}
	if err := query.Take(&row).Error; err != nil {
		return Record{}, httpx.NewError(http.StatusNotFound, "ASSET_NOT_FOUND", "素材不存在")
	}
	record := Record{ID: row.ID, AssetKey: row.AssetKey, Name: row.Name, Category: row.Category, AltText: row.AltText, Status: row.Status, Fit: row.Fit, Position: row.Position, FallbackAssetKey: row.FallbackAssetKey, CurrentVersion: row.CurrentVersion, UpdatedAt: row.UpdatedAt, FocalPoint: map[string]int{}, SafeArea: map[string]int{}, ApplicablePages: []string{}, Tags: []string{}, Versions: []Version{}}
	_ = json.Unmarshal(row.FocalPoint, &record.FocalPoint)
	_ = json.Unmarshal(row.SafeArea, &record.SafeArea)
	_ = json.Unmarshal(row.ApplicablePages, &record.ApplicablePages)
	var err error
	record.Versions, err = s.versions(ctx, row.ID)
	if err != nil {
		return Record{}, err
	}
	if err := s.db.WithContext(ctx).Table("asset_tag_links atl").Joins("JOIN asset_tags at ON at.id=atl.tag_id").Where("atl.asset_id=?", row.ID).Pluck("at.name", &record.Tags).Error; err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Update(ctx context.Context, assetID uuid.UUID, input CreateInput) (Record, error) {
	pages, _ := json.Marshal(input.ApplicablePages)
	focal, _ := json.Marshal(defaults(input.FocalPoint, map[string]int{"x": 50, "y": 50}))
	safe, _ := json.Marshal(defaults(input.SafeArea, map[string]int{"top": 8, "right": 8, "bottom": 8, "left": 8}))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"updated_at": time.Now().UTC(), "focal_point": focal, "safe_area": safe, "applicable_pages": pages}
		for key, value := range map[string]string{"asset_key": input.AssetKey, "name": input.Name, "alt_text": input.AltText, "fit": input.Fit, "position": input.Position, "fallback_asset_key": input.FallbackAssetKey} {
			if value != "" {
				updates[key] = value
			}
		}
		if input.Category != "" {
			var category struct{ ID uuid.UUID }
			if err := tx.Table("asset_categories").Where("key=?", input.Category).First(&category).Error; err != nil {
				return httpx.NewError(http.StatusBadRequest, "ASSET_CATEGORY_NOT_FOUND", "素材分类不存在")
			}
			updates["category_id"] = category.ID
		}
		if result := tx.Table("assets").Where("id=?", assetID).Updates(updates); result.Error != nil {
			return result.Error
		} else if result.RowsAffected == 0 {
			return httpx.NewError(http.StatusNotFound, "ASSET_NOT_FOUND", "素材不存在")
		}
		if input.Tags != nil {
			if err := tx.Exec("DELETE FROM asset_tag_links WHERE asset_id=?", assetID).Error; err != nil {
				return err
			}
			return s.bindTags(tx, assetID, input.Tags)
		}
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return s.Get(ctx, assetID.String())
}
func (s *Service) Create(ctx context.Context, actor uuid.UUID, input CreateInput, upload Upload) (Record, error) {
	if input.AssetKey == "" || input.Name == "" {
		return Record{}, httpx.NewError(http.StatusBadRequest, "INVALID_ASSET", "assetKey 和名称不能为空")
	}
	metadata, err := inspect(upload)
	if err != nil {
		return Record{}, err
	}
	digest := sha256.Sum256(upload.Data)
	hash := hex.EncodeToString(digest[:])
	var duplicate int64
	if err := s.db.WithContext(ctx).Table("asset_versions").Where("sha256=?", hash).Count(&duplicate).Error; err != nil {
		return Record{}, err
	}
	if duplicate > 0 {
		return Record{}, httpx.NewError(http.StatusConflict, "DUPLICATE_ASSET", "相同内容的素材已经存在")
	}
	objectKey := fmt.Sprintf("assets/%s/%s%s", time.Now().UTC().Format("2006/01"), uuid.NewString(), metadata.Extension)
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(upload.Data), int64(len(upload.Data)), metadata.MIME); err != nil {
		return Record{}, err
	}
	assetID, versionID := uuid.New(), uuid.New()
	focal := defaults(input.FocalPoint, map[string]int{"x": 50, "y": 50})
	safe := defaults(input.SafeArea, map[string]int{"top": 8, "right": 8, "bottom": 8, "left": 8})
	pages, _ := json.Marshal(input.ApplicablePages)
	focalJSON, _ := json.Marshal(focal)
	safeJSON, _ := json.Marshal(safe)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var categoryID *uuid.UUID
		if input.Category != "" {
			var category struct{ ID uuid.UUID }
			if err := tx.Table("asset_categories").Where("key=?", input.Category).First(&category).Error; err != nil {
				return httpx.NewError(http.StatusBadRequest, "ASSET_CATEGORY_NOT_FOUND", "素材分类不存在")
			}
			categoryID = &category.ID
		}
		asset := map[string]any{"id": assetID, "asset_key": input.AssetKey, "name": input.Name, "category_id": categoryID, "alt_text": input.AltText, "status": "draft", "fit": defaultString(input.Fit, "contain"), "position": defaultString(input.Position, "center"), "focal_point": focalJSON, "safe_area": safeJSON, "applicable_pages": pages, "fallback_asset_key": input.FallbackAssetKey, "current_version": 1, "created_by": actor, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}
		if err := tx.Table("assets").Create(asset).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.NewError(http.StatusConflict, "ASSET_KEY_EXISTS", "assetKey 已存在")
			}
			return err
		}
		riskJSON, _ := json.Marshal(metadata.Risks)
		version := map[string]any{"id": versionID, "asset_id": assetID, "version": 1, "object_key": objectKey, "public_url": "", "mime_type": metadata.MIME, "width": metadata.Width, "height": metadata.Height, "has_alpha": metadata.HasAlpha, "sha256": hash, "risk_flags": riskJSON, "note": "原始上传", "created_by": actor, "created_at": time.Now().UTC()}
		if err := tx.Table("asset_versions").Create(version).Error; err != nil {
			return err
		}
		return s.bindTags(tx, assetID, input.Tags)
	})
	if err != nil {
		_ = s.storage.Delete(context.Background(), objectKey)
		return Record{}, err
	}
	return s.Get(ctx, assetID.String())
}
func (s *Service) AddVersion(ctx context.Context, actor, assetID uuid.UUID, upload Upload, note string) (Record, error) {
	metadata, err := inspect(upload)
	if err != nil {
		return Record{}, err
	}
	digest := sha256.Sum256(upload.Data)
	hash := hex.EncodeToString(digest[:])
	objectKey := fmt.Sprintf("assets/%s/%s%s", time.Now().UTC().Format("2006/01"), uuid.NewString(), metadata.Extension)
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(upload.Data), int64(len(upload.Data)), metadata.MIME); err != nil {
		return Record{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset struct{ CurrentVersion int }
		if err := tx.Table("assets").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", assetID).First(&asset).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "ASSET_NOT_FOUND", "素材不存在")
		}
		var next int
		if err := tx.Table("asset_versions").Where("asset_id=?", assetID).Select("COALESCE(MAX(version),0)+1").Scan(&next).Error; err != nil {
			return err
		}
		riskJSON, _ := json.Marshal(metadata.Risks)
		if err := tx.Table("asset_versions").Create(map[string]any{"id": uuid.New(), "asset_id": assetID, "version": next, "object_key": objectKey, "mime_type": metadata.MIME, "width": metadata.Width, "height": metadata.Height, "has_alpha": metadata.HasAlpha, "sha256": hash, "risk_flags": riskJSON, "note": note, "created_by": actor, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Table("assets").Where("id=?", assetID).Updates(map[string]any{"current_version": next, "status": "draft", "updated_at": time.Now().UTC()}).Error
	})
	if err != nil {
		_ = s.storage.Delete(context.Background(), objectKey)
		return Record{}, err
	}
	return s.Get(ctx, assetID.String())
}
func (s *Service) Publish(ctx context.Context, assetID, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset struct{ CurrentVersion int }
		if err := tx.Table("assets").Where("id=?", assetID).First(&asset).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "ASSET_NOT_FOUND", "素材不存在")
		}
		if err := tx.Table("assets").Where("id=?", assetID).Updates(map[string]any{"status": "published", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Table("audit_logs").Create(map[string]any{"id": uuid.New(), "actor_id": actor, "actor_type": "user", "action": "asset.publish", "resource_type": "asset", "resource_id": assetID.String(), "after_json": map[string]any{"version": asset.CurrentVersion}, "created_at": time.Now().UTC()}).Error
	})
}
func (s *Service) Rollback(ctx context.Context, assetID, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset struct{ CurrentVersion int }
		if err := tx.Table("assets").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", assetID).First(&asset).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "ASSET_NOT_FOUND", "素材不存在")
		}
		if asset.CurrentVersion <= 1 {
			return httpx.NewError(http.StatusConflict, "NO_PREVIOUS_VERSION", "没有可回滚的历史版本")
		}
		next := asset.CurrentVersion - 1
		if err := tx.Table("assets").Where("id=?", assetID).Updates(map[string]any{"current_version": next, "status": "published", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Table("audit_logs").Create(map[string]any{"id": uuid.New(), "actor_id": actor, "actor_type": "user", "action": "asset.rollback", "resource_type": "asset", "resource_id": assetID.String(), "before_json": map[string]any{"version": asset.CurrentVersion}, "after_json": map[string]any{"version": next}, "created_at": time.Now().UTC()}).Error
	})
}
func (s *Service) Archive(ctx context.Context, assetID uuid.UUID) error {
	var count int64
	if err := s.db.WithContext(ctx).Table("asset_usage_records aur").Joins("JOIN asset_versions av ON av.id=aur.asset_version_id").Where("av.asset_id=? AND aur.historical=true", assetID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return httpx.NewError(http.StatusConflict, "ASSET_HAS_HISTORICAL_USAGE", "历史比赛或奖励正在引用该素材，不能归档")
	}
	return s.db.WithContext(ctx).Table("assets").Where("id=?", assetID).Updates(map[string]any{"status": "archived", "updated_at": time.Now().UTC()}).Error
}
func (s *Service) PublicManifest(ctx context.Context) (map[string]any, error) {
	var total int64
	if err := s.db.WithContext(ctx).Table("assets").Where("status='published'").Count(&total).Error; err != nil {
		return nil, err
	}
	size := int(total)
	if size < 1 {
		size = 1
	}
	page, err := s.List(ctx, "", "", "published", 1, size)
	if err != nil {
		return nil, err
	}
	slots, err := s.Slots(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"version": time.Now().UTC().Format("200601021504"), "assets": page.Items, "slots": slots}, nil
}
func (s *Service) OpenVersion(ctx context.Context, versionID uuid.UUID, variant string) (io.ReadCloser, string, error) {
	var row struct{ ObjectKey, MobileObjectKey, DarkObjectKey, DarkMobileObjectKey, MIMEType string }
	if err := s.db.WithContext(ctx).Table("asset_versions av").Select("av.object_key,av.mobile_object_key,av.dark_object_key,av.dark_mobile_object_key,av.mime_type").Joins("JOIN assets a ON a.id=av.asset_id").Where("av.id=? AND a.status='published'", versionID).First(&row).Error; err != nil {
		return nil, "", httpx.NewError(http.StatusNotFound, "ASSET_VERSION_NOT_FOUND", "素材版本不存在")
	}
	key := row.ObjectKey
	switch variant {
	case "mobile":
		if row.MobileObjectKey != "" {
			key = row.MobileObjectKey
		}
	case "dark":
		if row.DarkObjectKey != "" {
			key = row.DarkObjectKey
		}
	case "dark-mobile":
		if row.DarkMobileObjectKey != "" {
			key = row.DarkMobileObjectKey
		} else if row.DarkObjectKey != "" {
			key = row.DarkObjectKey
		} else if row.MobileObjectKey != "" {
			key = row.MobileObjectKey
		}
	}
	reader, err := s.storage.Open(ctx, key)
	return reader, row.MIMEType, err
}
func (s *Service) versions(ctx context.Context, assetID uuid.UUID) ([]Version, error) {
	var rows []struct {
		ID                                                                                                uuid.UUID
		Version                                                                                           int
		ObjectKey, PublicURL, MobileObjectKey, DarkObjectKey, DarkMobileObjectKey, MIMEType, SHA256, Note string
		Width, Height                                                                                     int
		HasAlpha                                                                                          bool
		RiskFlags                                                                                         []byte
		CreatedAt                                                                                         time.Time
	}
	if err := s.db.WithContext(ctx).Table("asset_versions").Where("asset_id=?", assetID).Order("version").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]Version, 0, len(rows))
	for _, row := range rows {
		risks := []string{}
		_ = json.Unmarshal(row.RiskFlags, &risks)
		url := row.PublicURL
		if url == "" {
			url = s.publicBaseURL + "/api/v1/public/asset-content/" + row.ID.String()
		}
		result = append(result, Version{ID: row.ID, Version: row.Version, URL: url, MobileURL: variantURL(row.MobileObjectKey, url, "mobile"), DarkURL: variantURL(row.DarkObjectKey, url, "dark"), DarkMobileURL: variantURL(row.DarkMobileObjectKey, url, "dark-mobile"), MIMEType: row.MIMEType, SHA256: row.SHA256, Note: row.Note, Width: row.Width, Height: row.Height, HasAlpha: row.HasAlpha, RiskFlags: risks, CreatedAt: row.CreatedAt})
	}
	return result, nil
}
func (s *Service) bindTags(tx *gorm.DB, assetID uuid.UUID, tags []string) error {
	for _, name := range tags {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		var tag struct{ ID uuid.UUID }
		if err := tx.Table("asset_tags").Where("name=?", name).First(&tag).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			tag.ID = uuid.New()
			if err := tx.Table("asset_tags").Create(map[string]any{"id": tag.ID, "name": name, "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := tx.Table("asset_tag_links").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"asset_id": assetID, "tag_id": tag.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Categories(ctx context.Context) ([]map[string]any, error) {
	var items []map[string]any
	return items, s.db.WithContext(ctx).Table("asset_categories").Order("name").Find(&items).Error
}
func (s *Service) CreateCategory(ctx context.Context, key, name, description string) (map[string]any, error) {
	record := map[string]any{"id": uuid.New(), "key": key, "name": name, "description": description, "created_at": time.Now().UTC()}
	return record, s.db.WithContext(ctx).Table("asset_categories").Create(record).Error
}
func (s *Service) Tags(ctx context.Context) ([]map[string]any, error) {
	var items []map[string]any
	return items, s.db.WithContext(ctx).Table("asset_tags").Order("name").Find(&items).Error
}
func (s *Service) CreateTag(ctx context.Context, name string) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "TAG_NAME_REQUIRED", "标签名称不能为空")
	}
	record := map[string]any{"id": uuid.New(), "name": name, "created_at": time.Now().UTC()}
	return record, s.db.WithContext(ctx).Table("asset_tags").Create(record).Error
}
func (s *Service) Slots(ctx context.Context) ([]Slot, error) {
	var slots []Slot
	if err := s.db.WithContext(ctx).Table("asset_slots").Order("page_key,name").Scan(&slots).Error; err != nil {
		return nil, err
	}
	for index := range slots {
		_ = s.db.WithContext(ctx).Table("asset_bindings").Where("slot_id=?", slots[index].ID).Order("CASE WHEN status='published' THEN 0 ELSE 1 END,priority DESC,created_at DESC").Scan(&slots[index].Bindings).Error
		if len(slots[index].Bindings) > 0 {
			_ = s.hydrateSlotAssetKeys(ctx, &slots[index], slots[index].Bindings[0])
		}
	}
	return slots, nil
}
func (s *Service) CreateSlot(ctx context.Context, input Slot) (Slot, error) {
	input.ID = uuid.New()
	input.Version = 1
	if input.Fit == "" {
		input.Fit = "contain"
	}
	if input.Position == "" {
		input.Position = "center"
	}
	input.Enabled = true
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := map[string]any{"id": input.ID, "slot_key": input.SlotKey, "name": input.Name, "page_key": input.PageKey, "fit": input.Fit, "position": input.Position, "enabled": true, "version": 1, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}
		if err := tx.Table("asset_slots").Create(record).Error; err != nil {
			return err
		}
		if input.AssetKey == "" {
			return httpx.NewError(http.StatusBadRequest, "ASSET_KEY_REQUIRED", "槽位必须绑定默认素材")
		}
		return saveSlotBinding(ctx, tx, input)
	})
	return input, err
}
func (s *Service) UpdateSlot(ctx context.Context, slotID uuid.UUID, input Slot) (Slot, error) {
	updates := map[string]any{"updated_at": time.Now().UTC(), "version": gorm.Expr("version+1")}
	for key, value := range map[string]string{"slot_key": input.SlotKey, "name": input.Name, "page_key": input.PageKey, "fit": input.Fit, "position": input.Position} {
		if value != "" {
			updates[key] = value
		}
	}
	updates["enabled"] = input.Enabled
	input.ID = slotID
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if result := tx.Table("asset_slots").Where("id=?", slotID).Updates(updates); result.Error != nil {
			return result.Error
		} else if result.RowsAffected == 0 {
			return httpx.NewError(http.StatusNotFound, "ASSET_SLOT_NOT_FOUND", "素材槽位不存在")
		}
		if input.AssetKey == "" {
			return nil
		}
		return saveSlotBinding(ctx, tx, input)
	}); err != nil {
		return Slot{}, err
	}
	var slot Slot
	if err := s.db.WithContext(ctx).Table("asset_slots").Where("id=?", slotID).First(&slot).Error; err != nil {
		return Slot{}, err
	}
	var bindings []Binding
	_ = s.db.WithContext(ctx).Table("asset_bindings").Where("slot_id=?", slotID).Order("priority DESC,created_at DESC").Scan(&bindings).Error
	if len(bindings) > 0 {
		slot.Bindings = bindings
		_ = s.hydrateSlotAssetKeys(ctx, &slot, bindings[0])
	}
	return slot, nil
}

func saveSlotBinding(ctx context.Context, tx *gorm.DB, input Slot) error {
	if input.AssetKey == "" {
		return httpx.NewError(http.StatusBadRequest, "ASSET_KEY_REQUIRED", "槽位必须绑定默认素材")
	}
	resolve := func(key string) (*uuid.UUID, error) {
		if key == "" {
			return nil, nil
		}
		var row struct{ ID uuid.UUID }
		err := tx.WithContext(ctx).Table("asset_versions v").Select("v.id").Joins("JOIN assets a ON a.id=v.asset_id AND a.current_version=v.version").Where("a.asset_key=? AND a.status<>'archived'", key).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusBadRequest, "ASSET_NOT_FOUND", "槽位引用的素材不存在："+key)
		}
		if err != nil {
			return nil, err
		}
		return &row.ID, nil
	}
	light, err := resolve(input.AssetKey)
	if err != nil {
		return err
	}
	mobile, err := resolve(input.MobileAssetKey)
	if err != nil {
		return err
	}
	dark, err := resolve(input.DarkAssetKey)
	if err != nil {
		return err
	}
	darkMobile, err := resolve(input.DarkMobileAssetKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := tx.Table("asset_bindings").Where("slot_id=? AND status='published'", input.ID).Update("status", "archived").Error; err != nil {
		return err
	}
	return tx.Table("asset_bindings").Create(map[string]any{"id": uuid.New(), "slot_id": input.ID, "scope_type": "platform", "light_desktop_version_id": light, "light_mobile_version_id": mobile, "dark_desktop_version_id": dark, "dark_mobile_version_id": darkMobile, "priority": 0, "status": "published", "created_at": now}).Error
}

func (s *Service) hydrateSlotAssetKeys(ctx context.Context, slot *Slot, binding Binding) error {
	ids := []*uuid.UUID{binding.LightDesktopVersionID, binding.LightMobileVersionID, binding.DarkDesktopVersionID, binding.DarkMobileVersionID}
	keys := make([]string, len(ids))
	for index, id := range ids {
		if id == nil {
			continue
		}
		if err := s.db.WithContext(ctx).Table("asset_versions v").Select("a.asset_key").Joins("JOIN assets a ON a.id=v.asset_id").Where("v.id=?", *id).Take(&keys[index]).Error; err != nil {
			return err
		}
	}
	slot.AssetKey, slot.MobileAssetKey, slot.DarkAssetKey, slot.DarkMobileAssetKey = keys[0], keys[1], keys[2], keys[3]
	return nil
}

type metadata struct {
	MIME, Extension string
	Width, Height   int
	HasAlpha        bool
	Risks           []string
}

func inspect(upload Upload) (metadata, error) {
	if len(upload.Data) == 0 || int64(len(upload.Data)) > maxFileSize {
		return metadata{}, httpx.NewError(http.StatusBadRequest, "INVALID_FILE_SIZE", "素材文件为空或超过 25 MB")
	}
	detected := http.DetectContentType(upload.Data[:min(512, len(upload.Data))])
	extension := strings.ToLower(filepath.Ext(upload.Name))
	allowed := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/avif": ".avif", "image/svg+xml": ".svg"}
	expected, ok := allowed[detected]
	if !ok && extension == ".svg" {
		detected = "image/svg+xml"
		expected = ".svg"
		ok = true
	}
	if !ok {
		return metadata{}, httpx.NewError(http.StatusBadRequest, "UNSUPPORTED_ASSET_TYPE", "仅支持 PNG、JPEG、WebP、AVIF 和安全 SVG")
	}
	if detected == "image/svg+xml" {
		lower := strings.ToLower(string(upload.Data))
		for _, pattern := range []string{"<script", "<foreignobject", "javascript:", "onload=", "onerror=", "xlink:href=\"http", "href=\"http"} {
			if strings.Contains(lower, pattern) {
				return metadata{}, httpx.NewError(http.StatusBadRequest, "UNSAFE_SVG", "SVG 包含脚本、事件或外部资源")
			}
		}
		return metadata{MIME: detected, Extension: expected, Width: 1, Height: 1, HasAlpha: true}, nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(upload.Data))
	if err != nil {
		if detected == "image/avif" {
			width, height := avifDimensions(upload.Data)
			if width == 0 || height == 0 {
				return metadata{}, httpx.NewError(http.StatusBadRequest, "INVALID_IMAGE", "无法读取 AVIF 尺寸")
			}
			config.Width = width
			config.Height = height
		} else {
			return metadata{}, httpx.NewError(http.StatusBadRequest, "INVALID_IMAGE", "图片数据损坏或格式与扩展名不符")
		}
	}
	if int64(config.Width)*int64(config.Height) > maxPixels {
		return metadata{}, httpx.NewError(http.StatusBadRequest, "IMAGE_TOO_LARGE", "图片总像素超过 4000 万")
	}
	risks := []string{}
	if checkerboardRisk(upload.Data, detected) {
		risks = append(risks, "possible_baked_checkerboard")
	}
	hasAlpha := detected == "image/png" || detected == "image/webp" || detected == "image/avif"
	return metadata{MIME: detected, Extension: expected, Width: config.Width, Height: config.Height, HasAlpha: hasAlpha, Risks: risks}, nil
}
func inspectTeamAvatar(upload Upload) (metadata, error) {
	if len(upload.Data) == 0 || int64(len(upload.Data)) > maxTeamAvatarSize {
		return metadata{}, httpx.NewError(http.StatusRequestEntityTooLarge, "TEAM_AVATAR_TOO_LARGE", "战队头像不能超过 5 MB")
	}
	value, err := inspect(upload)
	if err != nil {
		return metadata{}, err
	}
	if value.MIME != "image/png" && value.MIME != "image/jpeg" && value.MIME != "image/webp" {
		return metadata{}, httpx.NewError(http.StatusUnsupportedMediaType, "UNSUPPORTED_TEAM_AVATAR_TYPE", "战队头像仅支持 PNG、JPEG 或 WebP")
	}
	if int64(value.Width)*int64(value.Height) > maxTeamAvatarPixels {
		return metadata{}, httpx.NewError(http.StatusBadRequest, "TEAM_AVATAR_DIMENSIONS_TOO_LARGE", "战队头像尺寸不能超过 4096×4096")
	}
	return value, nil
}
func avifDimensions(data []byte) (int, int) {
	for index := 0; index+16 < len(data); index++ {
		if string(data[index:index+4]) == "ispe" && index+12 < len(data) {
			return int(binary.BigEndian.Uint32(data[index+4 : index+8])), int(binary.BigEndian.Uint32(data[index+8 : index+12]))
		}
	}
	return 0, 0
}
func checkerboardRisk(data []byte, mime string) bool {
	return (mime == "image/png" || mime == "image/jpeg") && bytes.Count(data, []byte{0xf0, 0xf0, 0xf0}) > 64 && bytes.Count(data, []byte{0xff, 0xff, 0xff}) > 64
}
func defaults(value, fallback map[string]int) map[string]int {
	if len(value) == 0 {
		return fallback
	}
	return value
}
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func variantURL(key, base, variant string) string {
	if key == "" {
		return ""
	}
	return base + "?variant=" + variant
}
