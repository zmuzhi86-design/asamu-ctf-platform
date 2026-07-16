package instance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/competitionscope"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/queue"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db                        *gorm.DB
	stream                    *queue.Stream
	cfg                       config.Runtime
	hmacSecret, encryptionKey []byte
}

func NewService(db *gorm.DB, stream *queue.Stream, cfg config.Runtime, securityCfg config.Security) *Service {
	return &Service{db: db, stream: stream, cfg: cfg, hmacSecret: []byte(securityCfg.FlagHMACSecret), encryptionKey: securityCfg.FlagEncryptionKey}
}

type View struct {
	ID               uuid.UUID  `json:"id"`
	ChallengeID      uuid.UUID  `json:"challengeId"`
	ChallengeSlug    string     `json:"challengeSlug"`
	Status           Status     `json:"status"`
	AccessURL        string     `json:"accessUrl,omitempty"`
	Port             int        `json:"port,omitempty"`
	InternalPort     int        `json:"internalPort,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	RemainingSeconds int64      `json:"remainingSeconds"`
	ErrorCode        string     `json:"errorCode,omitempty"`
	ErrorMessage     string     `json:"errorMessage,omitempty"`
	Version          int64      `json:"version"`
	Generation       int        `json:"generation"`
}
type AdminView struct {
	ID              uuid.UUID  `json:"id"`
	ChallengeID     uuid.UUID  `json:"challengeId"`
	ChallengeSlug   string     `json:"challengeSlug"`
	ChallengeTitle  string     `json:"challengeTitle"`
	OwnerScope      string     `json:"ownerScope"`
	OwnerID         uuid.UUID  `json:"ownerId"`
	OwnerName       string     `json:"ownerName"`
	CompetitionID   *uuid.UUID `json:"competitionId,omitempty"`
	CompetitionName string     `json:"competitionName,omitempty"`
	Status          Status     `json:"status"`
	AccessURL       string     `json:"accessUrl,omitempty"`
	HostPort        int        `json:"hostPort,omitempty"`
	InternalPort    int        `json:"internalPort,omitempty"`
	RuntimeProvider string     `json:"runtimeProvider"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	StoppedAt       *time.Time `json:"stoppedAt,omitempty"`
	ErrorCode       string     `json:"errorCode,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	Version         int64      `json:"version"`
	StatusVersion   int64      `json:"statusVersion"`
	Generation      int        `json:"generation"`
	OperationID     *uuid.UUID `json:"operationId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
type AdminOperationView struct {
	ID          uuid.UUID  `json:"id"`
	ActorID     uuid.UUID  `json:"actorId"`
	Operation   string     `json:"operation"`
	FromStatus  string     `json:"fromStatus"`
	ToStatus    string     `json:"toStatus"`
	Result      string     `json:"result"`
	ErrorCode   string     `json:"errorCode,omitempty"`
	RequestID   string     `json:"requestId,omitempty"`
	RequestedAt time.Time  `json:"requestedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}
type AdminDetailView struct {
	Instance   AdminView            `json:"instance"`
	Operations []AdminOperationView `json:"operations"`
}
type AdminRuntimeEventView struct {
	ID             uuid.UUID       `json:"id"`
	InstanceID     uuid.UUID       `json:"instanceId"`
	Type           string          `json:"type"`
	ProviderStatus string          `json:"providerStatus,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"createdAt"`
}
type AdminTransitionInput struct {
	Reason          string `json:"reason"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IP              string `json:"-"`
	UserAgent       string `json:"-"`
}
type AdminWorkerView struct {
	WorkerID           string    `json:"workerId"`
	Hostname           string    `json:"hostname"`
	Status             string    `json:"status"`
	LastErrorCode      string    `json:"lastErrorCode,omitempty"`
	Enabled            bool      `json:"enabled"`
	Draining           bool      `json:"draining"`
	CPUTotalMilli      int       `json:"cpuTotalMilli"`
	MemoryTotalMB      int       `json:"memoryTotalMb"`
	MaxInstances       int       `json:"maxInstances"`
	ActiveInstances    int       `json:"activeInstances"`
	ReservedCPUMilli   int       `json:"reservedCpuMilli"`
	ReservedMemoryMB   int       `json:"reservedMemoryMb"`
	CPUPercent         int       `json:"cpuPercent"`
	MemoryPercent      int       `json:"memoryPercent"`
	SupportedProtocols []string  `json:"supportedProtocols"`
	CachedImages       []string  `json:"cachedImages"`
	LastHeartbeat      time.Time `json:"lastHeartbeat"`
	RegisteredAt       time.Time `json:"registeredAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	Version            int64     `json:"version"`
}
type AdminWorkerDrainInput struct {
	Draining        bool   `json:"draining"`
	Reason          string `json:"reason"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IP, UserAgent   string `json:"-"`
}
type Scope struct{ CompetitionID, TeamID *uuid.UUID }
type operationPayload struct {
	JobID uuid.UUID `json:"jobId"`
}
type jobPayload struct {
	JobID uuid.UUID `json:"jobId"`
}

func (s *Service) Status(ctx context.Context, userID uuid.UUID, identifier string, scope Scope) (View, error) {
	challengeID, slug, err := s.resolveChallenge(ctx, identifier, scope.CompetitionID)
	if err != nil {
		return View{}, err
	}
	scope, err = s.resolveScope(ctx, userID, challengeID, scope, false)
	if err != nil {
		return View{}, err
	}
	instance, err := s.findCurrent(ctx, userID, challengeID, scope)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return View{ID: uuid.Nil, ChallengeID: challengeID, ChallengeSlug: slug, Status: Stopped, Version: 0}, nil
		}
		return View{}, err
	}
	return view(instance, slug), nil
}
func (s *Service) Start(ctx context.Context, userID uuid.UUID, identifier, idempotencyKey, requestID string, scope Scope) (View, error) {
	if err := s.ensureEnabled(); err != nil {
		return View{}, err
	}
	challengeID, slug, err := s.resolveChallenge(ctx, identifier, scope.CompetitionID)
	if err != nil {
		return View{}, err
	}
	scope, err = s.resolveScope(ctx, userID, challengeID, scope, false)
	if err != nil {
		return View{}, err
	}
	var created models.ChallengeInstance
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdempotencyKey(tx, userID, idempotencyKey); err != nil {
			return err
		}
		if existing, ok, err := s.idempotentForScope(tx, userID, idempotencyKey, "start", challengeID, scope); err != nil {
			return err
		} else if ok {
			created = existing
			return nil
		}
		if scope.CompetitionID != nil {
			if _, err := competitionscope.ActiveChallenge(ctx, tx, *scope.CompetitionID, challengeID, true); err != nil {
				return err
			}
		} else if err := ensureGlobalChallengeActive(tx, challengeID); err != nil {
			return err
		}
		if err := ensureWorkerAvailable(tx); err != nil {
			return err
		}
		ownerScope, ownerID := scopeOwner(userID, scope)
		// Quotas apply across all challenges owned by the same user or team. A
		// per-challenge lock alone lets simultaneous starts for different
		// challenges both observe the same remaining quota and overbook it.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "runtime-quota:"+ownerScope+":"+ownerID.String()).Error; err != nil {
			return err
		}
		lockKey := ownerScope + ":" + ownerID.String() + ":" + challengeID.String() + ":" + optionalScope(scope.CompetitionID)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey).Error; err != nil {
			return err
		}
		// Recheck after the legacy scope lock so a request already in flight on
		// an older API node can commit and still be replayed during an upgrade.
		if existing, ok, err := s.idempotentForScope(tx, userID, idempotencyKey, "start", challengeID, scope); err != nil {
			return err
		} else if ok {
			created = existing
			return nil
		}
		var active models.ChallengeInstance
		findErr := tx.Where("challenge_id=? AND owner_scope=? AND owner_id=? AND competition_id IS NOT DISTINCT FROM ? AND status IN ?", challengeID, ownerScope, ownerID, scope.CompetitionID, []string{"pending", "pulling", "creating", "starting", "running", "restarting", "resetting", "stopping"}).First(&active).Error
		if findErr == nil {
			return httpx.NewError(http.StatusConflict, "INSTANCE_ALREADY_ACTIVE", "该题目已有活动环境")
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		runtimeCfg, runtimeRevisionID, err := s.runtimeForStart(tx, challengeID, scope.CompetitionID)
		if err != nil {
			return err
		}
		if err := s.enforceQuota(tx, ownerScope, ownerID, runtimeCfg); err != nil {
			return err
		}
		_, ciphertext, hmacValue, err := s.newFlag(runtimeCfg.FlagFormat)
		if err != nil {
			return err
		}
		created = models.ChallengeInstance{ID: uuid.New(), ChallengeID: challengeID, OwnerUserID: userID, OwnerTeamID: scope.TeamID, OwnerScope: ownerScope, OwnerID: ownerID, CompetitionID: scope.CompetitionID, RuntimeRevisionID: &runtimeRevisionID, RuntimeProvider: s.cfg.Provider, Status: string(Pending), InternalPort: runtimeCfg.InternalPort, Generation: 1, FlagCiphertext: ciphertext, FlagHMAC: hmacValue, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Version: 1}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := s.recordUsageStart(tx, created, runtimeCfg); err != nil {
			return err
		}
		return s.createJob(tx, &created, userID, "start", string(Stopped), string(Pending), idempotencyKey, requestID)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if existing, ok, replayErr := s.idempotentForScope(s.db.WithContext(ctx), userID, idempotencyKey, "start", challengeID, scope); replayErr != nil {
				return View{}, replayErr
			} else if ok {
				return view(existing, slug), nil
			}
		}
		return View{}, err
	}
	return view(created, slug), nil
}
func (s *Service) Restart(ctx context.Context, userID uuid.UUID, identifier, idempotencyKey, requestID string, scope Scope) (View, error) {
	return s.transition(ctx, userID, identifier, "restart", idempotencyKey, requestID, false, scope)
}
func (s *Service) Stop(ctx context.Context, userID uuid.UUID, identifier, idempotencyKey, requestID string, scope Scope) (View, error) {
	return s.transition(ctx, userID, identifier, "stop", idempotencyKey, requestID, false, scope)
}
func (s *Service) Reset(ctx context.Context, userID uuid.UUID, identifier, idempotencyKey, requestID string, scope Scope) (View, error) {
	return s.transition(ctx, userID, identifier, "reset", idempotencyKey, requestID, true, scope)
}
func (s *Service) transition(ctx context.Context, userID uuid.UUID, identifier, operation, idempotencyKey, requestID string, rotateFlag bool, scope Scope) (View, error) {
	if err := s.ensureEnabled(); err != nil {
		return View{}, err
	}
	challengeID, slug, err := s.resolveChallenge(ctx, identifier, scope.CompetitionID)
	if err != nil {
		return View{}, err
	}
	scope, err = s.resolveScope(ctx, userID, challengeID, scope, false)
	if err != nil {
		return View{}, err
	}
	var current models.ChallengeInstance
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdempotencyKey(tx, userID, idempotencyKey); err != nil {
			return err
		}
		if existing, ok, err := s.idempotentForScope(tx, userID, idempotencyKey, operation, challengeID, scope); err != nil {
			return err
		} else if ok {
			current = existing
			return nil
		}
		if operation != "stop" {
			if scope.CompetitionID != nil {
				if _, err := competitionscope.ActiveChallenge(ctx, tx, *scope.CompetitionID, challengeID, true); err != nil {
					return err
				}
			} else if err := ensureGlobalChallengeActive(tx, challengeID); err != nil {
				return err
			}
		}
		ownerScope, ownerID := scopeOwner(userID, scope)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", ownerScope+":"+ownerID.String()+":"+challengeID.String()+":"+optionalScope(scope.CompetitionID)).Error; err != nil {
			return err
		}
		if existing, ok, err := s.idempotentForScope(tx, userID, idempotencyKey, operation, challengeID, scope); err != nil {
			return err
		} else if ok {
			current = existing
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("challenge_id=? AND owner_scope=? AND owner_id=? AND competition_id IS NOT DISTINCT FROM ?", challengeID, ownerScope, ownerID, scope.CompetitionID).Order("created_at DESC").First(&current).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "没有可操作的环境")
		}
		next, stateErr := Next(operation, Status(current.Status))
		if stateErr != nil {
			return httpx.NewError(http.StatusConflict, "INVALID_INSTANCE_STATE", stateErr.Error())
		}
		before := current.Status
		updates := map[string]any{"status": string(next), "updated_at": time.Now().UTC(), "version": gorm.Expr("version + 1")}
		if rotateFlag {
			flagFormat, err := s.runtimeFlagFormat(tx, current)
			if err != nil {
				return err
			}
			_, ciphertext, hmacValue, err := s.newFlag(flagFormat)
			if err != nil {
				return err
			}
			updates["flag_ciphertext"] = ciphertext
			updates["flag_hmac"] = hmacValue
			updates["generation"] = gorm.Expr("generation + 1")
		}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		current.Status = string(next)
		current.Version++
		if rotateFlag {
			current.Generation++
			current.FlagCiphertext = updates["flag_ciphertext"].([]byte)
			current.FlagHMAC = updates["flag_hmac"].([]byte)
		}
		return s.createJob(tx, &current, userID, operation, before, string(next), idempotencyKey, requestID)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if existing, ok, replayErr := s.idempotentForScope(s.db.WithContext(ctx), userID, idempotencyKey, operation, challengeID, scope); replayErr != nil {
				return View{}, replayErr
			} else if ok {
				return view(existing, slug), nil
			}
		}
		return View{}, err
	}
	return view(current, slug), nil
}
func (s *Service) Extend(ctx context.Context, userID uuid.UUID, identifier string, seconds int, scope Scope) (View, error) {
	if err := s.ensureEnabled(); err != nil {
		return View{}, err
	}
	if seconds <= 0 {
		seconds = 1800
	}
	if time.Duration(seconds)*time.Second > s.cfg.MaxTTL {
		return View{}, httpx.NewError(http.StatusBadRequest, "TTL_LIMIT_EXCEEDED", "延长时间超过平台上限")
	}
	challengeID, slug, err := s.resolveChallenge(ctx, identifier, scope.CompetitionID)
	if err != nil {
		return View{}, err
	}
	scope, err = s.resolveScope(ctx, userID, challengeID, scope, true)
	if err != nil {
		return View{}, err
	}
	var instance models.ChallengeInstance
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if scope.CompetitionID != nil {
			if _, err := competitionscope.ActiveChallenge(ctx, tx, *scope.CompetitionID, challengeID, true); err != nil {
				return err
			}
		} else if err := ensureGlobalChallengeActive(tx, challengeID); err != nil {
			return err
		}
		ownerScope, ownerID := scopeOwner(userID, scope)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("challenge_id=? AND owner_scope=? AND owner_id=? AND competition_id IS NOT DISTINCT FROM ?", challengeID, ownerScope, ownerID, scope.CompetitionID).Order("created_at DESC").First(&instance).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "没有可延长的环境")
		}
		if Status(instance.Status) != Running {
			return httpx.NewError(http.StatusConflict, "INVALID_INSTANCE_STATE", "仅运行中的环境可以延长")
		}
		base := time.Now().UTC()
		if instance.ExpiresAt != nil && instance.ExpiresAt.After(base) {
			base = *instance.ExpiresAt
		}
		maximum := time.Now().UTC().Add(s.cfg.MaxTTL)
		next := base.Add(time.Duration(seconds) * time.Second)
		if next.After(maximum) {
			next = maximum
		}
		if err := tx.Model(&instance).Updates(map[string]any{"expires_at": next, "updated_at": time.Now().UTC(), "version": gorm.Expr("version+1")}).Error; err != nil {
			return err
		}
		instance.ExpiresAt = &next
		instance.Version++
		return nil
	})
	if err != nil {
		return View{}, err
	}
	return view(instance, slug), nil
}
func (s *Service) AdminList(ctx context.Context, page, pageSize int) (httpx.Page[AdminView], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Count(&total).Error; err != nil {
		return httpx.Page[AdminView]{}, err
	}
	var rows []AdminView
	query := adminInstanceQuery(s.db.WithContext(ctx))
	if err := query.Order("ci.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return httpx.Page[AdminView]{}, err
	}
	for index := range rows {
		rows[index].AccessURL = normalizeAccessURL(rows[index].AccessURL)
	}
	return httpx.Page[AdminView]{Items: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}
func (s *Service) AdminDetail(ctx context.Context, instanceID uuid.UUID) (AdminDetailView, error) {
	var instance AdminView
	if err := adminInstanceQuery(s.db.WithContext(ctx)).Where("ci.id=?", instanceID).Take(&instance).Error; err != nil {
		return AdminDetailView{}, httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "环境不存在")
	}
	instance.AccessURL = normalizeAccessURL(instance.AccessURL)
	var operations []AdminOperationView
	if err := s.db.WithContext(ctx).Table("instance_operations").Select("id,actor_id,operation,from_status,to_status,result,error_code,request_id,requested_at,finished_at").Where("instance_id=?", instanceID).Order("requested_at DESC").Limit(50).Scan(&operations).Error; err != nil {
		return AdminDetailView{}, err
	}
	return AdminDetailView{Instance: instance, Operations: operations}, nil
}
func (s *Service) AdminLogs(ctx context.Context, instanceID uuid.UUID) ([]AdminRuntimeEventView, error) {
	var exists int64
	if err := s.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Where("id=?", instanceID).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "环境不存在")
	}
	var events []AdminRuntimeEventView
	if err := s.db.WithContext(ctx).Table("instance_runtime_events").Select("id,instance_id,type,provider_status,payload,created_at").Where("instance_id=?", instanceID).Order("created_at DESC").Limit(200).Scan(&events).Error; err != nil {
		return nil, err
	}
	for index := range events {
		events[index].Payload = redactRuntimePayload(events[index].Payload)
	}
	return events, nil
}
func (s *Service) AdminWorkers(ctx context.Context) ([]AdminWorkerView, error) {
	var rows []models.RuntimeWorkerNode
	if err := s.db.WithContext(ctx).Order("last_heartbeat DESC, worker_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AdminWorkerView, 0, len(rows))
	for _, row := range rows {
		item := adminWorkerView(row, time.Now().UTC())
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) AdminSetWorkerDrain(ctx context.Context, actorID uuid.UUID, workerID, requestID string, input AdminWorkerDrainInput) (AdminWorkerView, error) {
	if err := validateWorkerDrainInput(workerID, &input); err != nil {
		return AdminWorkerView{}, err
	}
	var row models.RuntimeWorkerNode
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "worker_id=?", workerID).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "WORKER_NOT_FOUND", "Worker 节点不存在")
		}
		if row.Version != input.ExpectedVersion {
			return httpx.NewError(http.StatusConflict, "WORKER_VERSION_CONFLICT", "Worker 状态已变化，请刷新后重试")
		}
		before, _ := json.Marshal(map[string]any{"draining": row.Draining, "enabled": row.Enabled, "version": row.Version})
		now := time.Now().UTC()
		if err := tx.Model(&row).Updates(map[string]any{"draining": input.Draining, "updated_at": now, "version": gorm.Expr("version+1")}).Error; err != nil {
			return err
		}
		row.Draining = input.Draining
		row.Version++
		row.UpdatedAt = now
		after, _ := json.Marshal(map[string]any{"draining": row.Draining, "enabled": row.Enabled, "version": row.Version, "reason": input.Reason})
		actor := actorID
		action := "runtime.worker.resume"
		if input.Draining {
			action = "runtime.worker.drain"
		}
		return tx.Create(&models.AuditLog{ID: uuid.New(), ActorID: &actor, ActorType: "user", Action: action, ResourceType: "runtime_worker", ResourceID: workerID, IP: input.IP, UserAgent: input.UserAgent, RequestID: requestID, BeforeJSON: before, AfterJSON: after, CreatedAt: now}).Error
	})
	if err != nil {
		return AdminWorkerView{}, err
	}
	return adminWorkerView(row, time.Now().UTC()), nil
}
func (s *Service) AdminTransition(ctx context.Context, actorID, instanceID uuid.UUID, operation, idempotencyKey, requestID string, input AdminTransitionInput) (View, error) {
	if err := s.ensureEnabled(); err != nil {
		return View{}, err
	}
	if err := validateAdminTransitionInput(operation, &input); err != nil {
		return View{}, err
	}
	var result models.ChallengeInstance
	var slug string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdempotencyKey(tx, actorID, idempotencyKey); err != nil {
			return err
		}
		if existing, ok, err := s.adminIdempotent(tx, actorID, instanceID, operation, idempotencyKey); err != nil {
			return err
		} else if ok {
			result = existing
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, "id=?", instanceID).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "环境不存在")
		}
		if existing, ok, err := s.adminIdempotent(tx, actorID, instanceID, operation, idempotencyKey); err != nil {
			return err
		} else if ok {
			result = existing
			return nil
		}
		if result.Version != input.ExpectedVersion {
			return httpx.NewError(http.StatusConflict, "INSTANCE_VERSION_CONFLICT", "实例状态已变化，请刷新后重试")
		}
		next, stateErr := Next(operation, Status(result.Status))
		if stateErr != nil {
			return httpx.NewError(http.StatusConflict, "INVALID_INSTANCE_STATE", stateErr.Error())
		}
		before := result.Status
		beforeJSON, _ := json.Marshal(map[string]any{"status": before, "version": result.Version, "generation": result.Generation})
		updates := map[string]any{"status": string(next), "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}
		if operation == "reset" {
			flagFormat, err := s.runtimeFlagFormat(tx, result)
			if err != nil {
				return err
			}
			_, ciphertext, hmacValue, err := s.newFlag(flagFormat)
			if err != nil {
				return err
			}
			updates["flag_ciphertext"] = ciphertext
			updates["flag_hmac"] = hmacValue
			updates["generation"] = gorm.Expr("generation+1")
		}
		if err := tx.Model(&result).Updates(updates).Error; err != nil {
			return err
		}
		result.Status = string(next)
		result.Version++
		if operation == "reset" {
			result.Generation++
		}
		if err := s.createJob(tx, &result, actorID, operation, before, string(next), idempotencyKey, requestID); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(map[string]any{"status": result.Status, "version": result.Version, "generation": result.Generation, "operationId": result.OperationID, "reason": input.Reason})
		actor := actorID
		return tx.Create(&models.AuditLog{ID: uuid.New(), ActorID: &actor, ActorType: "user", Action: "instance.admin." + operation, ResourceType: "challenge_instance", ResourceID: instanceID.String(), IP: input.IP, UserAgent: input.UserAgent, RequestID: requestID, BeforeJSON: beforeJSON, AfterJSON: afterJSON, CreatedAt: time.Now().UTC()}).Error
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return View{}, err
		}
		existing, ok, replayErr := s.adminIdempotent(s.db.WithContext(ctx), actorID, instanceID, operation, idempotencyKey)
		if replayErr != nil {
			return View{}, replayErr
		}
		if !ok {
			return View{}, err
		}
		result = existing
	}
	_ = s.db.WithContext(ctx).Table("challenges").Where("id=?", result.ChallengeID).Pluck("slug", &slug).Error
	return view(result, slug), nil
}

// ExpireInstance is the narrow system-only transition used by the runtime worker.
// It deliberately does not accept arbitrary operations or administrator metadata.
func (s *Service) ExpireInstance(ctx context.Context, actorID, instanceID uuid.UUID, idempotencyKey, requestID string) (View, error) {
	if err := s.ensureEnabled(); err != nil {
		return View{}, err
	}
	var result models.ChallengeInstance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdempotencyKey(tx, actorID, idempotencyKey); err != nil {
			return err
		}
		if existing, ok, err := s.idempotentForInstance(tx, actorID, idempotencyKey, "expire", instanceID); err != nil {
			return err
		} else if ok {
			result = existing
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, "id=?", instanceID).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "INSTANCE_NOT_FOUND", "环境不存在")
		}
		if existing, ok, err := s.idempotentForInstance(tx, actorID, idempotencyKey, "expire", instanceID); err != nil {
			return err
		} else if ok {
			result = existing
			return nil
		}
		next, stateErr := Next("expire", Status(result.Status))
		if stateErr != nil {
			return httpx.NewError(http.StatusConflict, "INVALID_INSTANCE_STATE", stateErr.Error())
		}
		before := result.Status
		if err := tx.Model(&result).Updates(map[string]any{"status": string(next), "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		result.Status = string(next)
		result.Version++
		return s.createJob(tx, &result, actorID, "expire", before, string(next), idempotencyKey, requestID)
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return View{}, err
		}
		existing, ok, replayErr := s.idempotentForInstance(s.db.WithContext(ctx), actorID, idempotencyKey, "expire", instanceID)
		if replayErr != nil {
			return View{}, replayErr
		}
		if !ok {
			return View{}, err
		}
		result = existing
	}
	var slug string
	_ = s.db.WithContext(ctx).Table("challenges").Where("id=?", result.ChallengeID).Pluck("slug", &slug).Error
	return view(result, slug), nil
}

func validateAdminTransitionInput(operation string, input *AdminTransitionInput) error {
	if operation != "stop" && operation != "reset" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_INSTANCE_OPERATION", "不支持的管理员实例操作")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	length := utf8.RuneCountInString(input.Reason)
	if length < 4 || length > 500 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_OPERATION_REASON", "操作理由须为 4 至 500 个字符")
	}
	if input.ExpectedVersion < 1 {
		return httpx.NewError(http.StatusBadRequest, "EXPECTED_VERSION_REQUIRED", "必须提供有效的实例版本")
	}
	if utf8.RuneCountInString(input.UserAgent) > 500 {
		input.UserAgent = string([]rune(input.UserAgent)[:500])
	}
	return nil
}

var workerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func validateWorkerDrainInput(workerID string, input *AdminWorkerDrainInput) error {
	if !workerIDPattern.MatchString(workerID) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_WORKER_ID", "Worker ID 不合法")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if length := utf8.RuneCountInString(input.Reason); length < 4 || length > 500 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_OPERATION_REASON", "操作理由须为 4 至 500 个字符")
	}
	if input.ExpectedVersion < 1 {
		return httpx.NewError(http.StatusBadRequest, "EXPECTED_VERSION_REQUIRED", "必须提供有效的 Worker 版本")
	}
	if utf8.RuneCountInString(input.UserAgent) > 500 {
		input.UserAgent = string([]rune(input.UserAgent)[:500])
	}
	return nil
}

func adminWorkerView(row models.RuntimeWorkerNode, now time.Time) AdminWorkerView {
	protocols, images := []string{}, []string{}
	_ = json.Unmarshal(row.SupportedProtocols, &protocols)
	_ = json.Unmarshal(row.CachedImages, &images)
	status := row.Status
	if !row.Enabled {
		status = "disabled"
	} else if row.LastHeartbeat.Before(now.Add(-90*time.Second)) || row.Status == "offline" {
		status = "offline"
	} else if row.Draining {
		status = "draining"
	}
	cpuPercent, memoryPercent := 0, 0
	if row.CPUTotalMilli > 0 {
		cpuPercent = min(100, row.ReservedCPUMilli*100/row.CPUTotalMilli)
	}
	if row.MemoryTotalMB > 0 {
		memoryPercent = min(100, row.ReservedMemoryMB*100/row.MemoryTotalMB)
	}
	return AdminWorkerView{WorkerID: row.WorkerID, Hostname: row.Hostname, Status: status, Enabled: row.Enabled, Draining: row.Draining, CPUTotalMilli: row.CPUTotalMilli, MemoryTotalMB: row.MemoryTotalMB, MaxInstances: row.MaxInstances, ActiveInstances: row.ActiveInstances, ReservedCPUMilli: row.ReservedCPUMilli, ReservedMemoryMB: row.ReservedMemoryMB, CPUPercent: cpuPercent, MemoryPercent: memoryPercent, SupportedProtocols: protocols, CachedImages: images, LastErrorCode: row.LastErrorCode, LastHeartbeat: row.LastHeartbeat, RegisteredAt: row.RegisteredAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func (s *Service) adminIdempotent(tx *gorm.DB, actorID, instanceID uuid.UUID, operation, key string) (models.ChallengeInstance, bool, error) {
	var runtimeOperation models.RuntimeOperation
	if err := tx.Where("requested_by=? AND idempotency_key=?", actorID, key).First(&runtimeOperation).Error; err == nil {
		if runtimeOperation.InstanceID != instanceID || runtimeOperation.OperationType != operation {
			return models.ChallengeInstance{}, false, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "幂等键已用于其他实例操作")
		}
		var instance models.ChallengeInstance
		if err := tx.First(&instance, "id=?", instanceID).Error; err != nil {
			return models.ChallengeInstance{}, false, err
		}
		return instance, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChallengeInstance{}, false, err
	}
	var record models.InstanceOperation
	if err := tx.Where("actor_id=? AND idempotency_key=?", actorID, key).First(&record).Error; err == nil {
		if record.InstanceID != instanceID || record.Operation != operation {
			return models.ChallengeInstance{}, false, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "幂等键已用于其他实例操作")
		}
		var instance models.ChallengeInstance
		if err := tx.First(&instance, "id=?", instanceID).Error; err != nil {
			return models.ChallengeInstance{}, false, err
		}
		return instance, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChallengeInstance{}, false, err
	}
	return models.ChallengeInstance{}, false, nil
}

func redactRuntimePayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	if len(payload) > 32*1024 {
		return json.RawMessage(`{"redacted":"payload too large"}`)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return json.RawMessage(`{"redacted":"invalid payload"}`)
	}
	var redact func(any) any
	redact = func(current any) any {
		switch typed := current.(type) {
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, item := range typed {
				lower := strings.ToLower(key)
				if strings.Contains(lower, "flag") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") || strings.Contains(lower, "ciphertext") || strings.Contains(lower, "apikey") || strings.Contains(lower, "api_key") || strings.Contains(lower, "privatekey") {
					result[key] = "[REDACTED]"
				} else {
					result[key] = redact(item)
				}
			}
			return result
		case []any:
			result := make([]any, len(typed))
			for index, item := range typed {
				result[index] = redact(item)
			}
			return result
		case string:
			return redactRuntimeString(typed)
		default:
			return current
		}
	}
	encoded, err := json.Marshal(redact(value))
	if err != nil {
		return json.RawMessage(`{"redacted":"invalid payload"}`)
	}
	return encoded
}

func redactRuntimeString(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "bearer ") || strings.Contains(lower, "password=") || strings.Contains(lower, "token=") || strings.Contains(lower, "secret=") {
		return "[REDACTED]"
	}
	for {
		lower = strings.ToLower(value)
		start := strings.Index(lower, "flag{")
		if start < 0 {
			break
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			value = value[:start] + "[REDACTED]"
			break
		}
		value = value[:start] + "[REDACTED]" + value[start+end+1:]
	}
	if utf8.RuneCountInString(value) > 2000 {
		value = string([]rune(value)[:2000]) + "…"
	}
	return value
}

func adminInstanceQuery(db *gorm.DB) *gorm.DB {
	return db.Table("challenge_instances ci").Select(`ci.id,ci.challenge_id,c.slug AS challenge_slug,c.title AS challenge_title,
ci.owner_scope,ci.owner_id,CASE WHEN ci.owner_scope='team' THEN COALESCE(t.name,'已删除战队') ELSE COALESCE(u.username,'已删除用户') END AS owner_name,
ci.competition_id,COALESCE(cp.name,'') AS competition_name,ci.status,ci.access_url,ci.host_port,ci.internal_port,ci.runtime_provider,
ci.started_at,ci.expires_at,ci.stopped_at,COALESCE(NULLIF(ci.last_error_code,''),ci.error_code) AS error_code,
LEFT(ci.last_error_message,500) AS error_message,ci.version,ci.status_version,ci.generation,ci.operation_id,ci.created_at,ci.updated_at`).
		Joins("JOIN challenges c ON c.id=ci.challenge_id").
		Joins("LEFT JOIN users u ON u.id=ci.owner_id AND ci.owner_scope='user'").
		Joins("LEFT JOIN teams t ON t.id=ci.owner_id AND ci.owner_scope='team'").
		Joins("LEFT JOIN competitions cp ON cp.id=ci.competition_id")
}
func (s *Service) ensureEnabled() error {
	if s.cfg.Enabled {
		return nil
	}
	return httpx.NewError(http.StatusServiceUnavailable, "RUNTIME_DISABLED", "动态靶场暂未启用")
}

func ensureWorkerAvailable(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.RuntimeWorkerNode{}).Where("enabled=true AND draining=false AND status='online' AND last_error_code='' AND last_heartbeat>?", time.Now().UTC().Add(-90*time.Second)).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NewError(http.StatusServiceUnavailable, "RUNTIME_WORKER_UNAVAILABLE", "没有可用的动态靶场 Worker，请联系管理员检查 Docker 服务")
	}
	return nil
}

// Serialize requests that share an actor and idempotency key before checking
// either operations table. This closes the gap where concurrent retries could
// both miss the existing row and one would fail with a state/unique conflict
// instead of replaying the first request.
func lockIdempotencyKey(tx *gorm.DB, actorID uuid.UUID, key string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "runtime-idempotency:"+actorID.String()+":"+key).Error
}

// New global runtime operations take a SHARE lock on the challenge so they
// cannot race an archive. Status, Stop, and idempotent replays deliberately do
// not call this helper, allowing users to inspect and clean up older instances.
func ensureGlobalChallengeActive(tx *gorm.DB, challengeID uuid.UUID) error {
	var challenge models.Challenge
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
		Select("id").
		Where("id=? AND status='published' AND is_dynamic=true", challengeID).
		Take(&challenge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.NewError(http.StatusNotFound, "DYNAMIC_CHALLENGE_NOT_FOUND", "动态题目不存在或已归档")
		}
		return err
	}
	return nil
}

func (s *Service) resolveChallenge(ctx context.Context, identifier string, competitionID *uuid.UUID) (uuid.UUID, string, error) {
	var row struct {
		ID   uuid.UUID
		Slug string
	}
	query := s.db.WithContext(ctx).Table("challenges c").Select("c.id,c.slug")
	if competitionID == nil {
		// Publication is rechecked under a transaction lock only for new
		// mutating operations. Keeping archived dynamic challenges resolvable
		// lets Status/Stop and idempotent retries finish safely after archival.
		query = query.Where("(c.id::text=? OR c.slug=?) AND c.is_dynamic=true", identifier, identifier)
	} else {
		query = query.Joins("JOIN competition_challenge_snapshots cs ON cs.challenge_id=c.id").
			Joins("JOIN competition_snapshots snapshot ON snapshot.id=cs.competition_snapshot_id").
			Joins("JOIN competitions competition ON competition.current_snapshot_id=snapshot.id").
			Joins("JOIN challenge_revisions revision ON revision.id=cs.challenge_revision_id").
			Where("competition.id=? AND (c.id::text=? OR c.slug=?) AND revision.is_dynamic=true", *competitionID, identifier, identifier)
	}
	err := query.First(&row).Error
	if err != nil {
		return uuid.Nil, "", httpx.NewError(http.StatusNotFound, "DYNAMIC_CHALLENGE_NOT_FOUND", "动态题目不存在")
	}
	return row.ID, row.Slug, nil
}
func (s *Service) findCurrent(ctx context.Context, userID, challengeID uuid.UUID, scope Scope) (models.ChallengeInstance, error) {
	var instance models.ChallengeInstance
	ownerScope, ownerID := scopeOwner(userID, scope)
	err := s.db.WithContext(ctx).Where("challenge_id=? AND owner_scope=? AND owner_id=? AND competition_id IS NOT DISTINCT FROM ?", challengeID, ownerScope, ownerID, scope.CompetitionID).Order("created_at DESC").First(&instance).Error
	return instance, err
}
func (s *Service) resolveScope(ctx context.Context, userID, challengeID uuid.UUID, scope Scope, requireActive bool) (Scope, error) {
	if scope.CompetitionID == nil {
		if scope.TeamID == nil {
			return scope, nil
		}
		var membership int64
		if err := s.db.WithContext(ctx).Table("team_members").Where("team_id=? AND user_id=?", *scope.TeamID, userID).Count(&membership).Error; err != nil {
			return Scope{}, err
		}
		if membership == 0 {
			return Scope{}, httpx.NewError(http.StatusForbidden, "NOT_TEAM_MEMBER", "当前用户不属于该战队")
		}
		return scope, nil
	}
	var competition models.Competition
	var err error
	if requireActive {
		competition, err = competitionscope.ActiveChallenge(ctx, s.db, *scope.CompetitionID, challengeID, false)
	} else {
		err = s.db.WithContext(ctx).First(&competition, "id=?", *scope.CompetitionID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Scope{}, httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
	}
	if err != nil {
		return Scope{}, err
	}
	if competition.Mode == "team" {
		participant, err := competitionscope.RegisteredTeam(ctx, s.db, *scope.CompetitionID, userID, scope.TeamID)
		if err != nil {
			if errors.Is(err, competitionscope.ErrAmbiguousTeam) {
				return Scope{}, httpx.NewError(http.StatusConflict, "TEAM_SCOPE_AMBIGUOUS", "当前比赛存在多个报名战队，请明确指定战队")
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return Scope{}, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "不在该比赛的报名阵容中")
			}
			return Scope{}, err
		}
		scope.TeamID = participant.TeamID
	} else {
		if scope.TeamID != nil {
			return Scope{}, httpx.NewError(http.StatusBadRequest, "TEAM_SCOPE_NOT_ALLOWED", "个人赛不能指定战队")
		}
		var count int64
		if err := s.db.WithContext(ctx).Table("competition_participants").Where("competition_id=? AND user_id=? AND team_id IS NULL AND status='registered'", *scope.CompetitionID, userID).Count(&count).Error; err != nil {
			return Scope{}, err
		}
		if count == 0 {
			return Scope{}, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "尚未报名该比赛")
		}
	}
	if !requireActive {
		var count int64
		if err := s.db.WithContext(ctx).Table("competition_challenges").Where("competition_id=? AND challenge_id=?", *scope.CompetitionID, challengeID).Count(&count).Error; err != nil {
			return Scope{}, err
		}
		if count == 0 {
			return Scope{}, httpx.NewError(http.StatusForbidden, "CHALLENGE_NOT_IN_COMPETITION", "题目不属于该比赛")
		}
	}
	return scope, nil
}
func optionalScope(value *uuid.UUID) string {
	if value == nil {
		return "global"
	}
	return value.String()
}

func scopeOwner(userID uuid.UUID, scope Scope) (string, uuid.UUID) {
	if scope.TeamID != nil {
		return "team", *scope.TeamID
	}
	return "user", userID
}

type effectiveQuota struct {
	MaxActiveInstances int
	MaxCPUMilli        int
	MaxMemoryMB        int
	MaxPIDs            int
	MaxTTLSeconds      int
}

type quotaPolicyRow struct {
	ScopeType          string
	ScopeID            *uuid.UUID
	MaxActiveInstances int
	MaxCPUMilli        int
	MaxMemoryMB        int
	MaxPIDs            int `gorm:"column:max_pids"`
	MaxTTLSeconds      int
}

type quotaOverrideRow struct {
	MaxActiveInstances *int
	MaxCPUMilli        *int
	MaxMemoryMB        *int
	MaxPIDs            *int `gorm:"column:max_pids"`
	MaxTTLSeconds      *int
}

func (s *Service) enforceQuota(tx *gorm.DB, ownerScope string, ownerID uuid.UUID, runtimeCfg models.ChallengeRuntimeConfig) error {
	quota := effectiveQuota{MaxActiveInstances: 2, MaxCPUMilli: 500, MaxMemoryMB: 512, MaxPIDs: 256, MaxTTLSeconds: 14400}
	if ownerScope == "team" {
		quota.MaxActiveInstances = 5
		quota.MaxCPUMilli = 1000
		quota.MaxMemoryMB = 1024
		quota.MaxPIDs = 512
	}
	var policies []quotaPolicyRow
	if err := tx.Table("runtime_quota_policies").Where("enabled=true AND scope_type IN ? AND (scope_id IS NULL OR scope_id=?)", []string{"platform", ownerScope}, ownerID).Order("CASE WHEN scope_type='platform' THEN 0 ELSE 1 END, scope_id NULLS FIRST").Find(&policies).Error; err != nil {
		return err
	}
	for _, policy := range policies {
		quota = effectiveQuota{MaxActiveInstances: policy.MaxActiveInstances, MaxCPUMilli: policy.MaxCPUMilli, MaxMemoryMB: policy.MaxMemoryMB, MaxPIDs: policy.MaxPIDs, MaxTTLSeconds: policy.MaxTTLSeconds}
	}
	var override quotaOverrideRow
	if err := tx.Table("runtime_quota_overrides").Where("scope_type=? AND scope_id=? AND starts_at<=now() AND (ends_at IS NULL OR ends_at>now())", ownerScope, ownerID).Order("starts_at DESC").Limit(1).Scan(&override).Error; err != nil {
		return err
	}
	if override.MaxActiveInstances != nil {
		quota.MaxActiveInstances = *override.MaxActiveInstances
	}
	if override.MaxCPUMilli != nil {
		quota.MaxCPUMilli = *override.MaxCPUMilli
	}
	if override.MaxMemoryMB != nil {
		quota.MaxMemoryMB = *override.MaxMemoryMB
	}
	if override.MaxPIDs != nil {
		quota.MaxPIDs = *override.MaxPIDs
	}
	if override.MaxTTLSeconds != nil {
		quota.MaxTTLSeconds = *override.MaxTTLSeconds
	}
	var active int64
	if err := tx.Model(&models.ChallengeInstance{}).Where("owner_scope=? AND owner_id=? AND status IN ?", ownerScope, ownerID, []string{"pending", "pulling", "creating", "starting", "running", "restarting", "resetting", "stopping"}).Count(&active).Error; err != nil {
		return err
	}
	if active >= int64(quota.MaxActiveInstances) {
		return httpx.NewError(http.StatusTooManyRequests, "RUNTIME_ACTIVE_QUOTA_EXCEEDED", "活动环境数量已达到配额上限")
	}
	if runtimeCfg.CPUMilli > quota.MaxCPUMilli || runtimeCfg.MemoryMB > quota.MaxMemoryMB || runtimeCfg.PIDsLimit > quota.MaxPIDs || runtimeCfg.TTLSeconds > quota.MaxTTLSeconds {
		return httpx.NewError(http.StatusUnprocessableEntity, "RUNTIME_RESOURCE_QUOTA_EXCEEDED", "题目运行资源超过当前配额")
	}
	return nil
}

func (s *Service) recordUsageStart(tx *gorm.DB, instance models.ChallengeInstance, runtimeCfg models.ChallengeRuntimeConfig) error {
	return tx.Exec(`INSERT INTO runtime_usage_counters(owner_scope,owner_id,day,starts,active_instances,reserved_cpu_milli,reserved_memory_mb,updated_at)
VALUES (?,?,CURRENT_DATE,1,1,?,?,now())
ON CONFLICT(owner_scope,owner_id,day) DO UPDATE SET
  starts=runtime_usage_counters.starts+1,
  active_instances=runtime_usage_counters.active_instances+1,
  reserved_cpu_milli=runtime_usage_counters.reserved_cpu_milli+EXCLUDED.reserved_cpu_milli,
  reserved_memory_mb=runtime_usage_counters.reserved_memory_mb+EXCLUDED.reserved_memory_mb,
  updated_at=now()`, instance.OwnerScope, instance.OwnerID, runtimeCfg.CPUMilli, runtimeCfg.MemoryMB).Error
}

func (s *Service) runtimeForStart(tx *gorm.DB, challengeID uuid.UUID, competitionID *uuid.UUID) (models.ChallengeRuntimeConfig, uuid.UUID, error) {
	var revision models.ChallengeRuntimeRevision
	query := tx.Table("challenge_runtime_revisions rr").Select("rr.*")
	if competitionID != nil {
		query = query.Joins("JOIN competition_challenge_snapshots cs ON cs.runtime_revision_id=rr.id").Joins("JOIN competitions c ON c.current_snapshot_id=cs.competition_snapshot_id").Where("c.id=? AND cs.challenge_id=?", *competitionID, challengeID)
	} else {
		query = query.Joins("JOIN challenges c ON c.current_published_revision_id=rr.challenge_revision_id").Where("c.id=?", challengeID)
	}
	if err := query.Take(&revision).Error; err != nil {
		return models.ChallengeRuntimeConfig{}, uuid.Nil, httpx.NewError(http.StatusUnprocessableEntity, "RUNTIME_REVISION_REQUIRED", "题目缺少已发布的运行时版本")
	}
	config := models.ChallengeRuntimeConfig{ChallengeID: challengeID, RegistryCredentialID: revision.RegistryCredentialID, ImageRef: revision.ImageRef, ImageDigest: revision.ImageDigest, InternalPort: revision.InternalPort, Protocol: revision.Protocol, FlagFormat: revision.FlagFormat, CPUMilli: revision.CPUMilli, MemoryMB: revision.MemoryMB, PIDsLimit: revision.PIDsLimit, DiskMB: revision.DiskMB, TTLSeconds: revision.TTLSeconds, MaxTTLSeconds: revision.MaxTTLSeconds, ReadOnlyRootFS: revision.ReadOnlyRootFS, EnvironmentTemplate: revision.EnvironmentTemplate, Enabled: true}
	return config, revision.ID, nil
}
func (s *Service) runtimeFlagFormat(tx *gorm.DB, instance models.ChallengeInstance) (string, error) {
	var flagFormat string
	if instance.RuntimeRevisionID != nil {
		if err := tx.Table("challenge_runtime_revisions").Where("id=?", *instance.RuntimeRevisionID).Pluck("flag_format", &flagFormat).Error; err != nil {
			return "", err
		}
	} else if err := tx.Table("challenge_runtime_configs").Where("challenge_id=?", instance.ChallengeID).Pluck("flag_format", &flagFormat).Error; err != nil {
		return "", err
	}
	if flagFormat == "" {
		flagFormat = "standard"
	}
	return flagFormat, nil
}
func (s *Service) idempotent(tx *gorm.DB, actorID uuid.UUID, key string) (models.ChallengeInstance, string, bool, error) {
	var runtimeOperation models.RuntimeOperation
	if err := tx.Where("requested_by=? AND idempotency_key=?", actorID, key).First(&runtimeOperation).Error; err == nil {
		var instance models.ChallengeInstance
		if err := tx.First(&instance, "id=?", runtimeOperation.InstanceID).Error; err != nil {
			return models.ChallengeInstance{}, "", false, err
		}
		return instance, runtimeOperation.OperationType, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChallengeInstance{}, "", false, err
	}
	var operation models.InstanceOperation
	err := tx.Where("actor_id=? AND idempotency_key=?", actorID, key).First(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChallengeInstance{}, "", false, nil
	}
	if err != nil {
		return models.ChallengeInstance{}, "", false, err
	}
	var instance models.ChallengeInstance
	if err := tx.First(&instance, "id=?", operation.InstanceID).Error; err != nil {
		return models.ChallengeInstance{}, "", false, err
	}
	return instance, operation.Operation, true, nil
}

func (s *Service) idempotentForScope(tx *gorm.DB, actorID uuid.UUID, key, expectedOperation string, challengeID uuid.UUID, scope Scope) (models.ChallengeInstance, bool, error) {
	instance, operation, ok, err := s.idempotent(tx, actorID, key)
	if err != nil || !ok {
		return instance, ok, err
	}
	if operation != expectedOperation || !instanceMatchesScope(instance, actorID, challengeID, scope) {
		return models.ChallengeInstance{}, false, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "幂等键已用于其他实例操作")
	}
	return instance, true, nil
}

func (s *Service) idempotentForInstance(tx *gorm.DB, actorID uuid.UUID, key, expectedOperation string, instanceID uuid.UUID) (models.ChallengeInstance, bool, error) {
	instance, operation, ok, err := s.idempotent(tx, actorID, key)
	if err != nil || !ok {
		return instance, ok, err
	}
	if operation != expectedOperation || instance.ID != instanceID {
		return models.ChallengeInstance{}, false, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "幂等键已用于其他实例操作")
	}
	return instance, true, nil
}

func instanceMatchesScope(instance models.ChallengeInstance, userID, challengeID uuid.UUID, scope Scope) bool {
	ownerScope, ownerID := scopeOwner(userID, scope)
	return instance.ChallengeID == challengeID && instance.OwnerScope == ownerScope && instance.OwnerID == ownerID && optionalUUIDsEqual(instance.CompetitionID, scope.CompetitionID)
}

func optionalUUIDsEqual(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
func (s *Service) createJob(tx *gorm.DB, instance *models.ChallengeInstance, actorID uuid.UUID, operation, from, to, idempotencyKey, requestID string) error {
	operationID := uuid.New()
	record := models.InstanceOperation{ID: operationID, InstanceID: instance.ID, ActorID: actorID, Operation: operation, FromStatus: from, ToStatus: to, Result: "pending", IdempotencyKey: idempotencyKey, RequestedAt: time.Now().UTC(), RequestID: requestID}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	operationPayloadJSON, _ := json.Marshal(map[string]any{"requestId": requestID, "fromStatus": from, "toStatus": to})
	requestedBy := actorID
	runtimeOperation := models.RuntimeOperation{ID: operationID, IdempotencyKey: idempotencyKey, InstanceID: instance.ID, OperationType: operation, RequestedBy: &requestedBy, Status: "pending", MaxRetries: 3, Payload: operationPayloadJSON, Result: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}
	if err := tx.Create(&runtimeOperation).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.ChallengeInstance{}).Where("id=?", instance.ID).Updates(map[string]any{"operation_id": operationID, "status_version": gorm.Expr("status_version+1")}).Error; err != nil {
		return err
	}
	instance.OperationID = &operationID
	instance.StatusVersion++
	payload, _ := json.Marshal(operationPayload{JobID: operationID})
	job := models.InstanceRuntimeJob{ID: uuid.New(), InstanceID: instance.ID, OperationID: operationID, Operation: operation, Status: "queued", Payload: payload, AvailableAt: time.Now().UTC(), CreatedAt: time.Now().UTC()}
	if err := tx.Create(&job).Error; err != nil {
		return err
	}
	eventPayload, _ := json.Marshal(jobPayload{JobID: job.ID})
	return tx.Table("outbox_events").Create(map[string]any{"id": uuid.New(), "aggregate_type": "runtime_operation", "aggregate_id": operationID, "event_type": "instance.operation.queued", "payload": eventPayload, "created_at": time.Now().UTC()}).Error
}
func (s *Service) newFlag(format string) (string, []byte, []byte, error) {
	var flag string
	if strings.EqualFold(strings.TrimSpace(format), "uuid") {
		flag = "flag{" + uuid.NewString() + "}"
	} else {
		random, err := security.RandomToken(24)
		if err != nil {
			return "", nil, nil, err
		}
		flag = "flag{cm_" + random + "}"
	}
	ciphertext, err := security.Encrypt([]byte(flag), s.encryptionKey)
	if err != nil {
		return "", nil, nil, err
	}
	return flag, ciphertext, security.FlagHMAC(s.hmacSecret, flag), nil
}
func view(instance models.ChallengeInstance, slug string) View {
	result := View{ID: instance.ID, ChallengeID: instance.ChallengeID, ChallengeSlug: slug, Status: Status(instance.Status), Port: instance.HostPort, InternalPort: instance.InternalPort, StartedAt: instance.StartedAt, ExpiresAt: instance.ExpiresAt, ErrorCode: instance.ErrorCode, ErrorMessage: instance.LastErrorMessage, Version: instance.Version, Generation: instance.Generation}
	if instance.Status == string(Running) || instance.Status == string(Restarting) {
		result.AccessURL = normalizeAccessURL(instance.AccessURL)
	}
	if instance.ExpiresAt != nil {
		remaining := time.Until(*instance.ExpiresAt).Seconds()
		if remaining > 0 {
			result.RemainingSeconds = int64(remaining)
		}
	}
	return result
}

func normalizeAccessURL(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "tcp://") || strings.HasPrefix(lower, "udp://") {
		return trimmed[len("tcp://"):]
	}
	return trimmed
}
func SanitizeRuntimeError(err error) string {
	var runtimeErr interface{ Error() string }
	if errors.As(err, &runtimeErr) {
		message := runtimeErr.Error()
		if len(message) > 120 {
			message = message[:120]
		}
		return message
	}
	return "runtime operation failed"
}
