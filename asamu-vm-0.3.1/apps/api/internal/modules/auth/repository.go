package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) RegistrationEnabled(ctx context.Context) (bool, error) {
	var version struct {
		SnapshotJSON json.RawMessage
	}
	err := r.db.WithContext(ctx).Table("platform_setting_versions").Select("snapshot_json").Where("status='published'").Order("published_at DESC,created_at DESC").Take(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return registrationEnabledFromSnapshot(version.SnapshotJSON)
}

func registrationEnabledFromSnapshot(raw json.RawMessage) (bool, error) {
	var snapshot struct {
		Features map[string]bool `json:"features"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return false, err
	}
	enabled, exists := snapshot.Features["registration"]
	if !exists {
		return true, nil
	}
	return enabled, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *models.User, profile *models.UserProfile, tokenHash string, expiresAt time.Time, ciphertext []byte) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		profile.UserID = user.ID
		if err := tx.Create(profile).Error; err != nil {
			return err
		}
		var role models.Role
		if err := tx.Where("key = ?", "user").First(&role).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
			return err
		}
		if err := tx.Table("email_verification_tokens").Create(map[string]any{"id": uuid.New(), "user_id": user.ID, "token_hash": tokenHash, "expires_at": expiresAt, "purpose": "verify_email"}).Error; err != nil {
			return err
		}
		return tx.Table("email_outbox").Create(map[string]any{"id": uuid.New(), "recipient": user.Email, "template_key": "verify_email", "payload_ciphertext": ciphertext}).Error
	})
}
func (r *Repository) UserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	return user, err
}

func (r *Repository) StoreVerification(ctx context.Context, userID uuid.UUID, email, hash string, expiresAt time.Time, ciphertext []byte) error {
	return r.storeMailToken(ctx, "email_verification_tokens", userID, email, hash, expiresAt, "verify_email", ciphertext)
}
func (r *Repository) StorePasswordReset(ctx context.Context, userID uuid.UUID, email, hash string, expiresAt time.Time, ciphertext []byte) error {
	return r.storeMailToken(ctx, "password_reset_tokens", userID, email, hash, expiresAt, "reset_password", ciphertext)
}
func (r *Repository) storeMailToken(ctx context.Context, table string, userID uuid.UUID, email, hash string, expiresAt time.Time, template string, ciphertext []byte) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		query := tx.Table(table).Where("user_id=? AND used_at IS NULL", userID)
		if table == "email_verification_tokens" {
			query = query.Where("purpose='verify_email'")
		}
		if err := query.Update("used_at", now).Error; err != nil {
			return err
		}
		values := map[string]any{"id": uuid.New(), "user_id": userID, "token_hash": hash, "expires_at": expiresAt}
		if table == "email_verification_tokens" {
			values["purpose"] = "verify_email"
		}
		if err := tx.Table(table).Create(values).Error; err != nil {
			return err
		}
		return tx.Table("email_outbox").Create(map[string]any{"id": uuid.New(), "recipient": email, "template_key": template, "payload_ciphertext": ciphertext}).Error
	})
}
func (r *Repository) ConsumeVerification(ctx context.Context, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ID, UserID uuid.UUID }
		if err := tx.Table("email_verification_tokens").Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash=? AND purpose='verify_email' AND used_at IS NULL AND expires_at>?", hash, now).Take(&row).Error; err != nil {
			return err
		}
		if err := tx.Table("email_verification_tokens").Where("id=?", row.ID).Update("used_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id=?", row.UserID).Updates(map[string]any{"email_verified_at": now, "updated_at": now}).Error
	})
}
func (r *Repository) StoreEmailChange(ctx context.Context, userID uuid.UUID, targetEmail, hash string, expiresAt time.Time, ciphertext []byte) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Where("email=? AND id<>?", targetEmail, userID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return gorm.ErrDuplicatedKey
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.User{}).Where("id=?", userID).Updates(map[string]any{"pending_email": targetEmail, "pending_email_requested_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Table("email_verification_tokens").Where("user_id=? AND purpose='change_email' AND used_at IS NULL", userID).Update("used_at", now).Error; err != nil {
			return err
		}
		if err := tx.Table("email_verification_tokens").Create(map[string]any{"id": uuid.New(), "user_id": userID, "token_hash": hash, "expires_at": expiresAt, "purpose": "change_email", "target_email": targetEmail}).Error; err != nil {
			return err
		}
		return tx.Table("email_outbox").Create(map[string]any{"id": uuid.New(), "recipient": targetEmail, "template_key": "change_email", "payload_ciphertext": ciphertext}).Error
	})
}
func (r *Repository) ConsumeEmailChange(ctx context.Context, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			ID, UserID  uuid.UUID
			TargetEmail string
		}
		if err := tx.Table("email_verification_tokens").Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash=? AND purpose='change_email' AND used_at IS NULL AND expires_at>?", hash, now).Take(&row).Error; err != nil {
			return err
		}
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id=?", row.UserID).Error; err != nil {
			return err
		}
		if user.PendingEmail == nil || !strings.EqualFold(*user.PendingEmail, row.TargetEmail) {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Table("email_verification_tokens").Where("id=?", row.ID).Update("used_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("id=?", row.UserID).Updates(map[string]any{"email": row.TargetEmail, "pending_email": nil, "pending_email_requested_at": nil, "email_verified_at": now, "token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", row.UserID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", row.UserID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "email_changed"}).Error
	})
}
func (r *Repository) ConsumePasswordReset(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ID, UserID uuid.UUID }
		if err := tx.Table("password_reset_tokens").Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash=? AND used_at IS NULL AND expires_at>?", tokenHash, now).Take(&row).Error; err != nil {
			return err
		}
		if err := tx.Table("password_reset_tokens").Where("id=?", row.ID).Update("used_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("id=?", row.UserID).Updates(map[string]any{"password_hash": passwordHash, "password_changed_at": now, "must_change_password": false, "token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", row.UserID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", row.UserID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "password_reset"}).Error
	})
}
func (r *Repository) UserByLogin(ctx context.Context, login string) (models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("(email = ? OR username = ?) AND deleted_at IS NULL", login, login).First(&user).Error
	return user, err
}
func (r *Repository) UserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	return user, err
}
func (r *Repository) RolesAndPermissions(ctx context.Context, userID uuid.UUID) ([]string, []string, error) {
	var roles, permissions []string
	if err := r.db.WithContext(ctx).Table("roles r").Select("r.key").Joins("JOIN user_roles ur ON ur.role_id = r.id").Where("ur.user_id = ?", userID).Scan(&roles).Error; err != nil {
		return nil, nil, err
	}
	if err := r.db.WithContext(ctx).Table("permissions p").Distinct("p.key").Joins("JOIN role_permissions rp ON rp.permission_id = p.id").Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").Where("ur.user_id = ?", userID).Scan(&permissions).Error; err != nil {
		return nil, nil, err
	}
	return roles, permissions, nil
}
func (r *Repository) StoreRefresh(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		var user models.User
		if err := tx.Select("token_version").First(&user, "id=?", token.UserID).Error; err != nil {
			return err
		}
		session := models.UserSession{ID: token.ID, UserID: token.UserID, RefreshTokenHash: token.TokenHash, TokenVersion: user.TokenVersion, IP: token.IP, UserAgent: token.UserAgent, CreatedAt: token.CreatedAt, LastSeenAt: token.CreatedAt, ExpiresAt: token.ExpiresAt}
		return tx.Create(&session).Error
	})
}
func (r *Repository) RefreshByHash(ctx context.Context, hash string) (models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error
	return token, err
}
func (r *Repository) RotateRefresh(ctx context.Context, hash string, now time.Time, replacement *models.RefreshToken) (models.RefreshToken, error) {
	var current models.RefreshToken
	replay := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", hash).First(&current).Error; err != nil {
			return err
		}
		if current.RevokedAt != nil || current.UsedAt != nil || !current.ExpiresAt.After(now) {
			family := current.FamilyID
			if err := tx.Model(&models.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", family).Update("revoked_at", now).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", current.UserID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "refresh_replay"}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.User{}).Where("id=?", current.UserID).Updates(map[string]any{"token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
				return err
			}
			replay = true
			return nil
		}
		if replacement == nil {
			return errors.New("refresh replacement is required")
		}
		if err := tx.Create(replacement).Error; err != nil {
			return err
		}
		var user models.User
		if err := tx.Select("token_version").First(&user, "id=?", current.UserID).Error; err != nil {
			return err
		}
		session := models.UserSession{ID: replacement.ID, UserID: replacement.UserID, RefreshTokenHash: replacement.TokenHash, TokenVersion: user.TokenVersion, IP: replacement.IP, UserAgent: replacement.UserAgent, CreatedAt: now, LastSeenAt: now, ExpiresAt: replacement.ExpiresAt}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.UserSession{}).Where("refresh_token_hash=? AND revoked_at IS NULL", hash).Updates(map[string]any{"revoked_at": now, "revoke_reason": "rotated", "last_seen_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&current).Updates(map[string]any{"used_at": now, "replaced_by_id": replacement.ID}).Error
	})
	if err == nil && replay {
		err = ErrRefreshReplay
	}
	return current, err
}
func (r *Repository) Revoke(ctx context.Context, hash string, allForUser bool) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.RefreshToken{}).Where("token_hash = ?", hash)
		sessionQuery := tx.Model(&models.UserSession{}).Where("refresh_token_hash=? AND revoked_at IS NULL", hash)
		if allForUser {
			var token models.RefreshToken
			if err := tx.Where("token_hash = ?", hash).First(&token).Error; err != nil {
				return err
			}
			query = tx.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", token.UserID)
			sessionQuery = tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", token.UserID)
			if err := tx.Model(&models.User{}).Where("id=?", token.UserID).Updates(map[string]any{"token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if err := query.Update("revoked_at", now).Error; err != nil {
			return err
		}
		return sessionQuery.Updates(map[string]any{"revoked_at": now, "revoke_reason": "logout"}).Error
	})
}
func (r *Repository) UpdatePassword(ctx context.Context, userID uuid.UUID, hash string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{"password_hash": hash, "password_changed_at": now, "must_change_password": false, "token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", userID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", userID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "password_changed"}).Error
	})
}

func (r *Repository) ValidateRefreshSession(ctx context.Context, hash string, userID uuid.UUID, tokenVersion int, now time.Time) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.UserSession{}).Where("refresh_token_hash=? AND user_id=? AND token_version=? AND revoked_at IS NULL AND expires_at>?", hash, userID, tokenVersion, now).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) RecordLogin(ctx context.Context, record map[string]any) {
	_ = r.db.WithContext(ctx).Table("login_records").Create(record).Error
}
