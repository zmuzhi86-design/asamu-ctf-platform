package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }
func (s *Service) Dashboard(ctx context.Context) (map[string]any, error) {
	result := map[string]any{}
	queries := map[string]string{"users": "users", "teams": "teams", "challenges": "challenges WHERE status='published'", "competitions": "competitions WHERE status='running'", "instances": "challenge_instances WHERE status IN ('starting','running','restarting','resetting')", "submissionsToday": "submissions WHERE created_at>=CURRENT_DATE", "openCheatCases": "cheat_cases WHERE status<>'closed'", "pendingWriteups": "writeups WHERE status='review'"}
	for key, table := range queries {
		var count int64
		if err := s.db.WithContext(ctx).Raw("SELECT count(*) FROM " + table).Scan(&count).Error; err != nil {
			return nil, err
		}
		result[key] = count
	}
	return result, nil
}

type UserRow struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	Status       string    `json:"status"`
	DisplayName  string    `json:"displayName"`
	Organization string    `json:"organization"`
	Roles        []string  `json:"roles"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Service) Users(ctx context.Context, search, status string, page, size int) (httpx.Page[UserRow], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("users u").Where("u.deleted_at IS NULL")
	if search != "" {
		query = query.Where("u.email ILIKE ? OR u.username ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" {
		query = query.Where("u.status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[UserRow]{}, err
	}
	var rows []UserRow
	if err := query.Select("u.id,u.email,u.username,u.status,up.display_name,up.organization_name AS organization,u.created_at").Joins("LEFT JOIN user_profiles up ON up.user_id=u.id").Order("u.created_at DESC").Offset((page - 1) * size).Limit(size).Scan(&rows).Error; err != nil {
		return httpx.Page[UserRow]{}, err
	}
	for index := range rows {
		_ = s.db.WithContext(ctx).Table("roles r").Joins("JOIN user_roles ur ON ur.role_id=r.id").Where("ur.user_id=?", rows[index].ID).Pluck("r.key", &rows[index].Roles).Error
	}
	return httpx.Page[UserRow]{Items: rows, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) SetUserStatus(ctx context.Context, actor, userID uuid.UUID, status, reason string) error {
	if status != "active" && status != "banned" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_USER_STATUS", "用户状态不合法")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, "id=?", userID).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		}
		if status == "banned" {
			if err := protectLastSuperAdmin(tx, userID); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if err := tx.Model(&before).Updates(map[string]any{"status": status, "token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if status == "banned" {
			if err := tx.Table("user_bans").Create(map[string]any{"id": uuid.New(), "user_id": userID, "reason": reason, "starts_at": time.Now().UTC(), "created_by": actor, "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
			_ = tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", userID).Update("revoked_at", now).Error
		} else {
			_ = tx.Table("user_bans").Where("user_id=? AND revoked_at IS NULL").Updates(map[string]any{"revoked_at": time.Now().UTC(), "revoked_by": actor}).Error
		}
		_ = tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", userID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "status_changed"}).Error
		return audit(tx, actor, "user.status", "user", userID.String(), map[string]any{"status": before.Status}, map[string]any{"status": status, "reason": reason})
	})
}
func (s *Service) AssignRole(ctx context.Context, actor, userID uuid.UUID, roleKey string, enabled bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role models.Role
		if err := tx.Where("key=?", roleKey).First(&role).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "ROLE_NOT_FOUND", "角色不存在")
		}
		if enabled {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserRole{UserID: userID, RoleID: role.ID}).Error; err != nil {
				return err
			}
		} else {
			if roleKey == "user" {
				return httpx.NewError(http.StatusConflict, "BASE_ROLE_REQUIRED", "不能移除基础用户角色")
			}
			if roleKey == "super_admin" {
				if err := protectLastSuperAdmin(tx, userID); err != nil {
					return err
				}
			}
			if err := tx.Where("user_id=? AND role_id=?", userID, role.ID).Delete(&models.UserRole{}).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.User{}).Where("id=?", userID).Updates(map[string]any{"token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", userID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", userID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "role_changed"}).Error; err != nil {
			return err
		}
		return audit(tx, actor, "user.role", "user", userID.String(), nil, map[string]any{"role": roleKey, "enabled": enabled})
	})
}

func protectLastSuperAdmin(tx *gorm.DB, target uuid.UUID) error {
	var targetCount int64
	if err := tx.Table("user_roles ur").Joins("JOIN roles r ON r.id=ur.role_id").Where("ur.user_id=? AND r.key='super_admin'", target).Count(&targetCount).Error; err != nil {
		return err
	}
	if targetCount == 0 {
		return nil
	}
	var active []uuid.UUID
	if err := tx.Table("users u").Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "u"}}).Select("u.id").Joins("JOIN user_roles ur ON ur.user_id=u.id").Joins("JOIN roles r ON r.id=ur.role_id").Where("r.key='super_admin' AND u.status='active' AND u.deleted_at IS NULL").Find(&active).Error; err != nil {
		return err
	}
	if len(active) <= 1 {
		return httpx.NewError(http.StatusConflict, "LAST_SUPER_ADMIN_PROTECTED", "不能禁用或移除最后一个超级管理员")
	}
	return nil
}
func (s *Service) Submissions(ctx context.Context, result string, page, size int) (httpx.Page[map[string]any], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("submissions s").Joins("JOIN users u ON u.id=s.user_id").Joins("JOIN challenges c ON c.id=s.challenge_id")
	if result != "" {
		query = query.Where("s.result=?", result)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	var items []map[string]any
	if err := query.Select("s.id,s.created_at,u.username,c.title AS challenge,s.result,s.awarded_score,s.ip,s.request_id,s.duration_ms").Order("s.created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	return httpx.Page[map[string]any]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) CheatCases(ctx context.Context, status string, page, size int) (httpx.Page[map[string]any], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("cheat_cases")
	if status != "" {
		query = query.Where("status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	var items []map[string]any
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	return httpx.Page[map[string]any]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) ResolveCheatCase(ctx context.Context, actor, caseID uuid.UUID, status, resolution, note string) error {
	if status != "open" && status != "investigating" && status != "closed" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_CASE_STATUS", "案件状态无效")
	}
	if len(resolution) > 5000 || len(note) > 5000 || status == "closed" && len(resolution) < 2 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_CASE_RESOLUTION", "结案时必须填写有效结论")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": status, "resolution": resolution}
		if status == "closed" {
			updates["closed_at"] = time.Now().UTC()
		}
		if err := tx.Table("cheat_cases").Where("id=?", caseID).Updates(updates).Error; err != nil {
			return err
		}
		if note != "" {
			if err := tx.Table("cheat_case_notes").Create(map[string]any{"id": uuid.New(), "case_id": caseID, "author_id": actor, "content": note, "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		return audit(tx, actor, "anticheat.review", "cheat_case", caseID.String(), nil, updates)
	})
}
func (s *Service) Audit(ctx context.Context, resourceType, actor string, page, size int) (httpx.Page[models.AuditLog], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Model(&models.AuditLog{})
	if resourceType != "" {
		query = query.Where("resource_type=?", resourceType)
	}
	if actor != "" {
		query = query.Where("actor_id::text=?", actor)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[models.AuditLog]{}, err
	}
	var items []models.AuditLog
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return httpx.Page[models.AuditLog]{}, err
	}
	return httpx.Page[models.AuditLog]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Announcements(ctx context.Context, status string, page, size int) (httpx.Page[map[string]any], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("announcements")
	if status != "" {
		query = query.Where("status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	var items []map[string]any
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return httpx.Page[map[string]any]{}, err
	}
	return httpx.Page[map[string]any]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}

type AnnouncementInput struct{ Type, Title, Content, Status string }

func (s *Service) CreateAnnouncement(ctx context.Context, actor uuid.UUID, input AnnouncementInput) (map[string]any, error) {
	if input.Type != "platform" && input.Type != "competition" && input.Type != "maintenance" {
		return nil, httpx.NewError(http.StatusBadRequest, "INVALID_ANNOUNCEMENT_TYPE", "公告类型无效")
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Status != "draft" && input.Status != "published" {
		return nil, httpx.NewError(http.StatusBadRequest, "INVALID_ANNOUNCEMENT_STATUS", "公告状态无效")
	}
	if len(input.Title) < 2 || len(input.Title) > 160 || len(input.Content) < 2 || len(input.Content) > 10000 {
		return nil, httpx.NewError(http.StatusBadRequest, "INVALID_ANNOUNCEMENT", "公告标题或内容长度无效")
	}
	item := map[string]any{"id": uuid.New(), "type": input.Type, "title": input.Title, "content": input.Content, "status": input.Status, "created_by": actor, "created_at": time.Now().UTC()}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("announcements").Create(item).Error; err != nil {
			return err
		}
		return audit(tx, actor, "announcement.create", "announcement", item["id"].(uuid.UUID).String(), nil, item)
	}); err != nil {
		return nil, err
	}
	return item, nil
}
func audit(tx *gorm.DB, actor uuid.UUID, action, resourceType, resourceID string, before, after any) error {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	return tx.Create(&models.AuditLog{ID: uuid.New(), ActorID: &actor, ActorType: "user", Action: action, ResourceType: resourceType, ResourceID: resourceID, BeforeJSON: beforeJSON, AfterJSON: afterJSON, CreatedAt: time.Now().UTC()}).Error
}
