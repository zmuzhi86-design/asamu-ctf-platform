package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrRefreshReplay = errors.New("refresh token replay detected")
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{3,32}$`)

type Service struct {
	repo            *Repository
	tokens          *security.TokenManager
	refreshTTL      time.Duration
	confirmationKey []byte
	publicBaseURL   string
}
type Session struct {
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"-"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	User                  UserView  `json:"user"`
}
type UserView struct {
	ID                 uuid.UUID `json:"id"`
	Email              string    `json:"email"`
	Username           string    `json:"username"`
	Status             string    `json:"status"`
	Roles              []string  `json:"roles"`
	Permissions        []string  `json:"permissions"`
	MustChangePassword bool      `json:"mustChangePassword"`
	EmailVerified      bool      `json:"emailVerified"`
	PendingEmail       string    `json:"pendingEmail,omitempty"`
}

func NewService(repo *Repository, cfg config.Security, publicBaseURL string) *Service {
	key := sha256.Sum256([]byte(cfg.ConfirmationTokenSecret))
	return &Service{repo: repo, tokens: security.NewTokenManager(cfg.JWTIssuer, cfg.JWTAccessSecret, cfg.JWTAccessTTL), refreshTTL: cfg.RefreshTokenTTL, confirmationKey: key[:], publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}
func (s *Service) TokenManager() *security.TokenManager { return s.tokens }
func (s *Service) Register(ctx context.Context, email, username, password, ip, userAgent string) (Session, error) {
	enabled, err := s.repo.RegistrationEnabled(ctx)
	if err != nil {
		return Session{}, err
	}
	if !enabled {
		return Session{}, httpx.NewError(http.StatusForbidden, "REGISTRATION_DISABLED", "平台当前未开放注册")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)
	if !strings.Contains(email, "@") || len(email) > 254 {
		return Session{}, httpx.NewError(http.StatusBadRequest, "INVALID_EMAIL", "邮箱格式不正确")
	}
	if !usernamePattern.MatchString(username) {
		return Session{}, httpx.NewError(http.StatusBadRequest, "INVALID_USERNAME", "用户名需为 3-32 位字母、数字、下划线或连字符")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return Session{}, httpx.NewError(http.StatusBadRequest, "WEAK_PASSWORD", "密码至少 10 位，且不能超过 128 位")
	}
	user := &models.User{ID: uuid.New(), Email: email, Username: username, PasswordHash: hash, Status: "active", TokenVersion: 1}
	profile := &models.UserProfile{DisplayName: username, Skills: []byte("[]"), Privacy: []byte("{}")}
	raw, tokenHash, ciphertext, err := s.emailToken(email, "verify_email", 24*time.Hour)
	if err != nil {
		return Session{}, err
	}
	_ = raw
	if err := s.repo.CreateUser(ctx, user, profile, tokenHash, time.Now().UTC().Add(24*time.Hour), ciphertext); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Session{}, httpx.NewError(http.StatusConflict, "ACCOUNT_EXISTS", "邮箱或用户名已被使用")
		}
		return Session{}, err
	}
	return s.createSession(ctx, *user, uuid.NewString(), ip, userAgent)
}
func (s *Service) Login(ctx context.Context, login, password, ip, userAgent string) (Session, error) {
	user, err := s.repo.UserByLogin(ctx, strings.TrimSpace(login))
	if err != nil || !security.VerifyPassword(user.PasswordHash, password) {
		s.repo.RecordLogin(ctx, map[string]any{"email": login, "success": false, "reason": "invalid_credentials", "ip": ip, "user_agent": userAgent})
		return Session{}, httpx.NewError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "账号或密码错误")
	}
	if user.Status == "banned" {
		return Session{}, httpx.NewError(http.StatusForbidden, "USER_BANNED", "账号已被封禁")
	}
	if user.Status != "active" {
		return Session{}, httpx.NewError(http.StatusForbidden, "USER_INACTIVE", "账号当前不可用")
	}
	s.repo.RecordLogin(ctx, map[string]any{"user_id": user.ID, "email": user.Email, "success": true, "ip": ip, "user_agent": userAgent})
	return s.createSession(ctx, user, uuid.NewString(), ip, userAgent)
}
func (s *Service) createSession(ctx context.Context, user models.User, familyID, ip, userAgent string) (Session, error) {
	roles, permissions, err := s.repo.RolesAndPermissions(ctx, user.ID)
	if err != nil {
		return Session{}, err
	}
	access, expires, err := s.tokens.Issue(user.ID.String(), user.TokenVersion, roles, permissions)
	if err != nil {
		return Session{}, err
	}
	refresh, err := security.RandomToken(48)
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	refreshExpires := now.Add(s.refreshTTL)
	token := &models.RefreshToken{ID: uuid.New(), UserID: user.ID, TokenHash: security.TokenHash(refresh), FamilyID: familyID, ExpiresAt: refreshExpires, CreatedAt: now, IP: ip, UserAgent: userAgent}
	if err := s.repo.StoreRefresh(ctx, token); err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, AccessTokenExpiresAt: expires, RefreshToken: refresh, RefreshTokenExpiresAt: refreshExpires, User: userView(user, roles, permissions)}, nil
}
func (s *Service) Refresh(ctx context.Context, raw, ip, userAgent string) (Session, error) {
	if raw == "" {
		return Session{}, httpx.NewError(http.StatusUnauthorized, "REFRESH_REQUIRED", "刷新令牌缺失")
	}
	tokenHash := security.TokenHash(raw)
	current, err := s.repo.RefreshByHash(ctx, tokenHash)
	if err != nil {
		return Session{}, httpx.NewError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "刷新令牌无效")
	}
	now := time.Now().UTC()
	if refreshTokenNeedsReplayRevocation(current, now) {
		if _, err := s.repo.RotateRefresh(ctx, tokenHash, now, nil); errors.Is(err, ErrRefreshReplay) {
			return Session{}, httpx.NewError(http.StatusUnauthorized, "REFRESH_REPLAY", "检测到刷新令牌重放，当前会话族已撤销")
		}
		return Session{}, httpx.NewError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "刷新令牌无效")
	}
	user, err := s.repo.UserByID(ctx, current.UserID)
	if err != nil || user.Status != "active" || s.repo.ValidateRefreshSession(ctx, tokenHash, current.UserID, user.TokenVersion, now) != nil {
		return Session{}, httpx.NewError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "刷新令牌无效")
	}
	nextRaw, err := security.RandomToken(48)
	if err != nil {
		return Session{}, err
	}
	next := &models.RefreshToken{ID: uuid.New(), UserID: current.UserID, TokenHash: security.TokenHash(nextRaw), FamilyID: current.FamilyID, ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now, IP: ip, UserAgent: userAgent}
	if _, err := s.repo.RotateRefresh(ctx, tokenHash, now, next); err != nil {
		if errors.Is(err, ErrRefreshReplay) {
			return Session{}, httpx.NewError(http.StatusUnauthorized, "REFRESH_REPLAY", "检测到刷新令牌重放，当前会话族已撤销")
		}
		return Session{}, httpx.NewError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "刷新令牌无效")
	}
	roles, permissions, err := s.repo.RolesAndPermissions(ctx, user.ID)
	if err != nil {
		return Session{}, err
	}
	access, expires, err := s.tokens.Issue(user.ID.String(), user.TokenVersion, roles, permissions)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, AccessTokenExpiresAt: expires, RefreshToken: nextRaw, RefreshTokenExpiresAt: next.ExpiresAt, User: userView(user, roles, permissions)}, nil
}

func refreshTokenNeedsReplayRevocation(token models.RefreshToken, now time.Time) bool {
	return token.RevokedAt != nil || token.UsedAt != nil || !token.ExpiresAt.After(now)
}
func (s *Service) Logout(ctx context.Context, raw string, all bool) error {
	if raw == "" {
		return nil
	}
	return s.repo.Revoke(ctx, security.TokenHash(raw), all)
}
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (UserView, error) {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return UserView{}, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	}
	roles, permissions, err := s.repo.RolesAndPermissions(ctx, user.ID)
	return userView(user, roles, permissions), err
}

func userView(user models.User, roles, permissions []string) UserView {
	pendingEmail := ""
	if user.PendingEmail != nil {
		pendingEmail = *user.PendingEmail
	}
	return UserView{ID: user.ID, Email: user.Email, Username: user.Username, Status: user.Status, Roles: roles, Permissions: permissions, MustChangePassword: user.MustChangePassword, EmailVerified: user.EmailVerifiedAt != nil, PendingEmail: pendingEmail}
}

func (s *Service) emailToken(email, template string, ttl time.Duration) (string, string, []byte, error) {
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", "", nil, err
	}
	path := "/verify-email?token="
	if template == "reset_password" {
		path = "/reset-password?token="
	}
	if template == "change_email" {
		path = "/confirm-email-change?token="
	}
	payload, err := json.Marshal(map[string]string{"token": raw, "url": s.publicBaseURL + path + raw})
	if err != nil {
		return "", "", nil, err
	}
	ciphertext, err := security.Encrypt(payload, s.confirmationKey)
	return raw, security.TokenHash(raw), ciphertext, err
}

func (s *Service) ResendVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return nil
	}
	_, hash, ciphertext, err := s.emailToken(user.Email, "verify_email", 24*time.Hour)
	if err != nil {
		return err
	}
	return s.repo.StoreVerification(ctx, user.ID, user.Email, hash, time.Now().UTC().Add(24*time.Hour), ciphertext)
}

func (s *Service) VerifyEmail(ctx context.Context, raw string) error {
	if len(raw) < 32 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_VERIFICATION_TOKEN", "验证链接无效或已过期")
	}
	if err := s.repo.ConsumeVerification(ctx, security.TokenHash(raw), time.Now().UTC()); err != nil {
		return httpx.NewError(http.StatusBadRequest, "INVALID_VERIFICATION_TOKEN", "验证链接无效或已过期")
	}
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.UserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || user.Status != "active" {
		return nil
	}
	_, hash, ciphertext, err := s.emailToken(user.Email, "reset_password", 30*time.Minute)
	if err != nil {
		return err
	}
	return s.repo.StorePasswordReset(ctx, user.ID, user.Email, hash, time.Now().UTC().Add(30*time.Minute), ciphertext)
}

func (s *Service) ResetPassword(ctx context.Context, raw, next string) error {
	if len(raw) < 32 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RESET_TOKEN", "重置链接无效或已过期")
	}
	hash, err := security.HashPassword(next)
	if err != nil {
		return httpx.NewError(http.StatusBadRequest, "WEAK_PASSWORD", "新密码至少 10 位且不能超过 128 位")
	}
	if err := s.repo.ConsumePasswordReset(ctx, security.TokenHash(raw), hash, time.Now().UTC()); err != nil {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RESET_TOKEN", "重置链接无效或已过期")
	}
	return nil
}
func (s *Service) RequestEmailChange(ctx context.Context, userID uuid.UUID, currentPassword, newEmail string) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !security.VerifyPassword(user.PasswordHash, currentPassword) {
		return httpx.NewError(http.StatusUnauthorized, "PASSWORD_MISMATCH", "当前密码不正确")
	}
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	if !strings.Contains(newEmail, "@") || len(newEmail) > 254 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_EMAIL", "邮箱格式不正确")
	}
	if strings.EqualFold(newEmail, user.Email) {
		return httpx.NewError(http.StatusConflict, "EMAIL_UNCHANGED", "新邮箱不能与当前邮箱相同")
	}
	_, hash, ciphertext, err := s.emailToken(newEmail, "change_email", 30*time.Minute)
	if err != nil {
		return err
	}
	if err := s.repo.StoreEmailChange(ctx, user.ID, newEmail, hash, time.Now().UTC().Add(30*time.Minute), ciphertext); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return httpx.NewError(http.StatusConflict, "ACCOUNT_EXISTS", "该邮箱已被使用")
		}
		return err
	}
	return nil
}

func (s *Service) ConfirmEmailChange(ctx context.Context, raw string) error {
	if len(raw) < 32 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_EMAIL_CHANGE_TOKEN", "确认链接无效或已过期")
	}
	if err := s.repo.ConsumeEmailChange(ctx, security.TokenHash(raw), time.Now().UTC()); err != nil {
		return httpx.NewError(http.StatusBadRequest, "INVALID_EMAIL_CHANGE_TOKEN", "确认链接无效或已过期")
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !security.VerifyPassword(user.PasswordHash, current) {
		return httpx.NewError(http.StatusUnauthorized, "PASSWORD_MISMATCH", "当前密码不正确")
	}
	hash, err := security.HashPassword(next)
	if err != nil {
		return httpx.NewError(http.StatusBadRequest, "WEAK_PASSWORD", "新密码至少 10 位")
	}
	return s.repo.UpdatePassword(ctx, userID, hash)
}

func (s *Service) ValidateAccess(ctx context.Context, userID uuid.UUID, tokenVersion int) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil || user.Status != "active" || user.TokenVersion != tokenVersion {
		return httpx.NewError(http.StatusUnauthorized, "SESSION_REVOKED", "登录状态已失效")
	}
	return nil
}
