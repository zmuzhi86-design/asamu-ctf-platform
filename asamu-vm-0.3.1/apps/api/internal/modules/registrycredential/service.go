package registrycredential

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db  *gorm.DB
	key []byte
}

func New(db *gorm.DB, key []byte) *Service { return &Service{db: db, key: key} }

type View struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	RegistryHost    string     `json:"registryHost"`
	Username        string     `json:"username"`
	Enabled         bool       `json:"enabled"`
	TokenConfigured bool       `json:"tokenConfigured"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Version         int64      `json:"version"`
}

type CreateInput struct {
	Name, RegistryHost, Username, Token string
	IP, UserAgent, RequestID            string
}

type UpdateInput struct {
	Name, Username, Token, Reason string
	Enabled                       *bool
	ExpectedVersion               int64
	IP, UserAgent, RequestID      string
}

type Lease struct {
	RegistryHost string    `json:"registryHost"`
	Username     string    `json:"username"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

var workerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

func (s *Service) List(ctx context.Context) ([]View, error) {
	var rows []models.RegistryCredential
	if err := s.db.WithContext(ctx).Order("name, registry_host").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]View, 0, len(rows))
	for _, row := range rows {
		items = append(items, view(row))
	}
	return items, nil
}

func (s *Service) Create(ctx context.Context, actorID uuid.UUID, input CreateInput) (View, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.RegistryHost = strings.ToLower(strings.TrimSpace(input.RegistryHost))
	input.Username = strings.TrimSpace(input.Username)
	if err := validateCreate(input); err != nil {
		return View{}, err
	}
	plaintext := []byte(input.Token)
	defer zero(plaintext)
	ciphertext, err := security.Encrypt(plaintext, s.key)
	if err != nil {
		return View{}, err
	}
	now := time.Now().UTC()
	actor := actorID
	row := models.RegistryCredential{ID: uuid.New(), Name: input.Name, RegistryHost: input.RegistryHost, Username: input.Username, EncryptedToken: ciphertext, TokenFingerprint: security.TokenHash(input.Token), Enabled: true, CreatedBy: &actor, CreatedAt: now, UpdatedAt: now, Version: 1}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return httpx.NewError(http.StatusConflict, "REGISTRY_HOST_EXISTS", "该镜像仓库已配置凭据")
			}
			return err
		}
		after, _ := json.Marshal(map[string]any{"name": row.Name, "registryHost": row.RegistryHost, "username": row.Username, "enabled": true, "tokenConfigured": true})
		return tx.Create(&models.AuditLog{ID: uuid.New(), ActorID: &actor, ActorType: "user", Action: "registry.credential.create", ResourceType: "registry_credential", ResourceID: row.ID.String(), IP: input.IP, UserAgent: input.UserAgent, RequestID: input.RequestID, AfterJSON: after, CreatedAt: now}).Error
	})
	if err != nil {
		return View{}, err
	}
	return view(row), nil
}

func (s *Service) Update(ctx context.Context, actorID, id uuid.UUID, input UpdateInput) (View, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Username = strings.TrimSpace(input.Username)
	input.Reason = strings.TrimSpace(input.Reason)
	if len(input.Name) < 1 || len(input.Name) > 100 || len(input.Username) < 1 || len(input.Username) > 200 || len(input.Reason) < 4 || len(input.Reason) > 500 || input.ExpectedVersion < 1 || (input.Token != "" && (len(input.Token) < 8 || len(input.Token) > 4096)) {
		return View{}, httpx.NewError(http.StatusBadRequest, "INVALID_REGISTRY_CREDENTIAL", "凭据名称、用户名、版本或变更原因不合法")
	}
	var row models.RegistryCredential
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id=?", id).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "REGISTRY_CREDENTIAL_NOT_FOUND", "镜像仓库凭据不存在")
		}
		if row.Version != input.ExpectedVersion {
			return httpx.NewError(http.StatusConflict, "REGISTRY_CREDENTIAL_VERSION_CONFLICT", "凭据已被修改，请刷新后重试")
		}
		before, _ := json.Marshal(map[string]any{"name": row.Name, "registryHost": row.RegistryHost, "username": row.Username, "enabled": row.Enabled, "version": row.Version})
		updates := map[string]any{"name": input.Name, "username": input.Username, "updated_at": time.Now().UTC(), "version": gorm.Expr("version+1")}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		tokenRotated := input.Token != ""
		if tokenRotated {
			plaintext := []byte(input.Token)
			defer zero(plaintext)
			ciphertext, err := security.Encrypt(plaintext, s.key)
			if err != nil {
				return err
			}
			updates["encrypted_token"] = ciphertext
			updates["token_fingerprint"] = security.TokenHash(input.Token)
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		row.Name, row.Username, row.UpdatedAt, row.Version = input.Name, input.Username, updates["updated_at"].(time.Time), row.Version+1
		if input.Enabled != nil {
			row.Enabled = *input.Enabled
		}
		after, _ := json.Marshal(map[string]any{"name": row.Name, "registryHost": row.RegistryHost, "username": row.Username, "enabled": row.Enabled, "version": row.Version, "tokenRotated": tokenRotated, "reason": input.Reason})
		actor := actorID
		return tx.Create(&models.AuditLog{ID: uuid.New(), ActorID: &actor, ActorType: "user", Action: "registry.credential.update", ResourceType: "registry_credential", ResourceID: row.ID.String(), IP: input.IP, UserAgent: input.UserAgent, RequestID: input.RequestID, BeforeJSON: before, AfterJSON: after, CreatedAt: row.UpdatedAt}).Error
	})
	if err != nil {
		return View{}, err
	}
	return view(row), nil
}

func (s *Service) Lease(ctx context.Context, credentialID, instanceID uuid.UUID, workerID, requestID, ip, userAgent string) (Lease, error) {
	if !workerIDPattern.MatchString(workerID) {
		return Lease{}, httpx.NewError(http.StatusUnauthorized, "WORKER_UNAUTHORIZED", "Worker 身份无效")
	}
	var result Lease
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var worker models.RuntimeWorkerNode
		if err := tx.First(&worker, "worker_id=?", workerID).Error; err != nil || !worker.Enabled || worker.Status != "online" || time.Since(worker.LastHeartbeat) > 90*time.Second {
			return httpx.NewError(http.StatusUnauthorized, "WORKER_UNAUTHORIZED", "Worker 未注册或不在线")
		}
		var instance models.ChallengeInstance
		if err := tx.First(&instance, "id=?", instanceID).Error; err != nil || instance.WorkerID != workerID || instance.RuntimeRevisionID == nil {
			return httpx.NewError(http.StatusForbidden, "REGISTRY_LEASE_FORBIDDEN", "实例不属于该 Worker")
		}
		var revision models.ChallengeRuntimeRevision
		if err := tx.Select("registry_credential_id,image_ref").First(&revision, "id=?", *instance.RuntimeRevisionID).Error; err != nil || revision.RegistryCredentialID == nil || *revision.RegistryCredentialID != credentialID {
			return httpx.NewError(http.StatusForbidden, "REGISTRY_LEASE_FORBIDDEN", "实例未绑定该镜像仓库凭据")
		}
		var row models.RegistryCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id=?", credentialID).Error; err != nil || !row.Enabled {
			return httpx.NewError(http.StatusNotFound, "REGISTRY_CREDENTIAL_NOT_FOUND", "镜像仓库凭据不存在或已停用")
		}
		if registryHostFromImage(revision.ImageRef) != row.RegistryHost {
			return httpx.NewError(http.StatusForbidden, "REGISTRY_LEASE_FORBIDDEN", "镜像引用与仓库凭据不匹配")
		}
		plaintext, err := security.Decrypt(row.EncryptedToken, s.key)
		if err != nil {
			return httpx.NewError(http.StatusInternalServerError, "REGISTRY_CREDENTIAL_DECRYPT_FAILED", "镜像仓库凭据无法解密")
		}
		defer zero(plaintext)
		now := time.Now().UTC()
		result = Lease{RegistryHost: row.RegistryHost, Username: row.Username, Token: string(plaintext), ExpiresAt: now.Add(time.Minute)}
		if err := tx.Model(&row).Update("last_used_at", now).Error; err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{"workerId": workerID, "instanceId": instanceID, "expiresAt": result.ExpiresAt})
		return tx.Create(&models.AuditLog{ID: uuid.New(), ActorType: "worker", Action: "registry.credential.lease", ResourceType: "registry_credential", ResourceID: row.ID.String(), IP: ip, UserAgent: userAgent, RequestID: requestID, AfterJSON: after, CreatedAt: now}).Error
	})
	return result, err
}

func view(row models.RegistryCredential) View {
	return View{ID: row.ID, Name: row.Name, RegistryHost: row.RegistryHost, Username: row.Username, Enabled: row.Enabled, TokenConfigured: len(row.EncryptedToken) > 0, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func validateCreate(input CreateInput) error {
	if len(input.Name) < 1 || len(input.Name) > 100 || len(input.Username) < 1 || len(input.Username) > 200 || len(input.Token) < 8 || len(input.Token) > 4096 || !validRegistryHost(input.RegistryHost) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_REGISTRY_CREDENTIAL", "镜像仓库地址、名称、用户名或 token 不合法")
	}
	return nil
}

func validRegistryHost(value string) bool {
	if value == "" || len(value) > 255 || strings.Contains(value, "://") || strings.ContainsAny(value, "/?#@ \\") {
		return false
	}
	host := value
	if strings.Count(value, ":") == 1 {
		var port string
		host, port, _ = net.SplitHostPort(value)
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return false
		}
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
				return false
			}
		}
	}
	return true
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func registryHostFromImage(imageRef string) string {
	first, _, hasPath := strings.Cut(strings.TrimSpace(imageRef), "/")
	if !hasPath {
		return "docker.io"
	}
	first = strings.ToLower(first)
	if first == "localhost" || strings.Contains(first, ".") || strings.Contains(first, ":") {
		return first
	}
	return "docker.io"
}
