package organization

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/validation"
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"time"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Organization struct {
	ID                            uuid.UUID `json:"id"`
	Slug, Name, Type, Description string
	OwnerID                       uuid.UUID `json:"ownerId"`
	MemberCount                   int64     `json:"memberCount"`
	CreatedAt                     time.Time
}

func (s *Service) List(ctx context.Context, page, size int) (httpx.Page[Organization], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var total int64
	if err := s.db.WithContext(ctx).Table("organizations").Count(&total).Error; err != nil {
		return httpx.Page[Organization]{}, err
	}
	var items []Organization
	if err := s.db.WithContext(ctx).Table("organizations o").Select("o.*,(SELECT count(*) FROM organization_members om WHERE om.organization_id=o.id)::bigint AS member_count").Order("o.name").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		return httpx.Page[Organization]{}, err
	}
	return httpx.Page[Organization]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Create(ctx context.Context, owner uuid.UUID, name, kind, description string) (Organization, error) {
	if name == "" {
		return Organization{}, httpx.NewError(http.StatusBadRequest, "INVALID_ORGANIZATION", "组织名称不能为空")
	}
	org := Organization{ID: uuid.New(), Slug: validation.Slug(name), Name: name, Type: kind, Description: description, OwnerID: owner, CreatedAt: time.Now().UTC()}
	if org.Type == "" {
		org.Type = "school"
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("organizations").Create(org).Error; err != nil {
			return err
		}
		return tx.Table("organization_members").Create(map[string]any{"organization_id": org.ID, "user_id": owner, "role": "admin", "joined_at": time.Now().UTC()}).Error
	})
	org.MemberCount = 1
	return org, err
}
func (s *Service) Join(ctx context.Context, userID, organizationID uuid.UUID) error {
	return s.db.WithContext(ctx).Table("organization_members").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"organization_id": organizationID, "user_id": userID, "role": "member", "joined_at": time.Now().UTC()}).Error
}
