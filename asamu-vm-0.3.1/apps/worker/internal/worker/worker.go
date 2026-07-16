package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	instancemodule "asamu.local/platform/api/internal/modules/instance"
	"asamu.local/platform/api/internal/platform/queue"
	"asamu.local/platform/api/internal/platform/security"
	runtimeprovider "asamu.local/platform/api/worker/internal/runtime"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Worker struct {
	db            *gorm.DB
	stream        *queue.Stream
	provider      runtimeprovider.Provider
	cfg           config.Runtime
	encryptionKey []byte
	logger        *zap.Logger
	workerID      string
	hostname      string
}

type jobPayload struct {
	JobID uuid.UUID `json:"jobId"`
}

type operationFailure struct {
	code      string
	message   string
	cause     error
	job       models.InstanceRuntimeJob
	instance  models.ChallengeInstance
	retryable bool
}

var errRuntimePortPoolExhausted = errors.New("runtime port pool exhausted")

const heartbeatImageListTimeout = 10 * time.Second

const syncRuntimePortPoolSQL = `INSERT INTO runtime_port_pool(worker_id,protocol,host_port)
SELECT ?,?,generated.host_port
FROM generate_series(?::integer, ?::integer) AS generated(host_port)
ON CONFLICT(worker_id,protocol,host_port) DO UPDATE SET enabled=true`

func (e *operationFailure) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.code, e.cause)
	}
	return e.code
}
func (e *operationFailure) Unwrap() error { return e.cause }

func New(db *gorm.DB, stream *queue.Stream, provider runtimeprovider.Provider, cfg config.Runtime, encryptionKey []byte, logger *zap.Logger) *Worker {
	if logger == nil {
		logger = zap.NewNop()
	}
	hostname, _ := os.Hostname()
	workerID := cfg.WorkerID
	if workerID == "" {
		workerID = hostname
	}
	if workerID == "" {
		workerID = "worker-" + uuid.NewString()
	}
	if hostname == "" {
		hostname = workerID
	}
	return &Worker{db: db, stream: stream, provider: provider, cfg: cfg, encryptionKey: encryptionKey, logger: logger, workerID: workerID, hostname: hostname}
}
func (w *Worker) Run(ctx context.Context) error {
	if err := w.stream.Ensure(ctx); err != nil {
		return err
	}
	// Reconcile Docker state before the first heartbeat. Renewing every active
	// lease first would keep stale leases alive after a reinstall or an
	// unexpected Docker cleanup, which can make a completely idle worker report
	// PORT_EXHAUSTED until a later maintenance cycle.
	if err := w.Reconcile(ctx); err != nil {
		// Never recycle active ports when Docker state could not be verified.
		// Heartbeats keep the leases safe and the next maintenance cycle retries.
		w.logger.Warn("runtime_initial_reconcile_failed", zap.Error(err))
	} else if err := w.MaintainLeases(ctx); err != nil {
		// Lease maintenance must not prevent the Worker from registering. The
		// periodic maintenance loop retries it after the first healthy heartbeat.
		w.logger.Warn("runtime_initial_lease_maintenance_failed", zap.Error(err))
	}
	if err := w.CleanupOrphans(ctx); err != nil {
		w.logger.Warn("runtime_initial_orphan_cleanup_failed", zap.Error(err))
	}
	if err := w.Heartbeat(ctx); err != nil {
		return err
	}
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		w.runHeartbeat(heartbeatCtx, 30*time.Second)
	}()
	defer func() {
		stopHeartbeat()
		<-heartbeatDone
		w.markOffline()
	}()
	outboxTicker := time.NewTicker(time.Second)
	maintenanceTicker := time.NewTicker(30 * time.Second)
	defer outboxTicker.Stop()
	defer maintenanceTicker.Stop()
	if err := w.PublishOutbox(ctx); err != nil {
		w.logger.Warn("outbox_initial_publish_failed", zap.Error(err))
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-outboxTicker.C:
			_ = w.PublishOutbox(ctx)
		case <-maintenanceTicker.C:
			if err := w.ExpireDue(ctx); err != nil {
				w.logger.Warn("runtime_expiry_scan_failed", zap.Error(err))
			}
			if err := w.Reconcile(ctx); err != nil {
				w.logger.Warn("runtime_reconcile_failed", zap.Error(err))
			} else if err := w.MaintainLeases(ctx); err != nil {
				w.logger.Warn("runtime_lease_maintenance_failed", zap.Error(err))
			}
			if err := w.CleanupOrphans(ctx); err != nil {
				w.logger.Warn("runtime_orphan_cleanup_failed", zap.Error(err))
			}
			if job, messageID, err := w.stream.ClaimStale(ctx, 2*time.Minute); err != nil {
				if !w.rejectMalformedMessage(ctx, messageID, err) {
					w.logger.Warn("queue_reclaim_failed", zap.Error(err))
				}
			} else if messageID != "" {
				w.process(ctx, job, messageID)
			}
		default:
			job, messageID, err := w.stream.Receive(ctx, 5*time.Second)
			if err != nil {
				if w.rejectMalformedMessage(ctx, messageID, err) {
					continue
				}
				w.logger.Error("queue_receive_failed", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
			if messageID == "" {
				continue
			}
			w.process(ctx, job, messageID)
		}
	}
}

func (w *Worker) runHeartbeat(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Heartbeat(ctx); err != nil && ctx.Err() == nil {
				w.logger.Warn("worker_heartbeat_failed", zap.String("worker_id", w.workerID), zap.Error(err))
			}
		}
	}
}

func (w *Worker) rejectMalformedMessage(ctx context.Context, messageID string, err error) bool {
	if messageID == "" || !errors.Is(err, queue.ErrMalformedJob) {
		return false
	}
	w.logger.Warn("queue_message_rejected", zap.String("message_id", messageID), zap.Error(err))
	if ackErr := w.stream.Ack(ctx, messageID); ackErr != nil {
		w.logger.Error("queue_message_reject_ack_failed", zap.String("message_id", messageID), zap.Error(ackErr))
	}
	return true
}

func (w *Worker) process(ctx context.Context, job queue.Job, messageID string) {
	if job.Type != "instance.operation" {
		_ = w.stream.Ack(ctx, messageID)
		return
	}
	accepted, reason, err := w.canProcess(ctx, job)
	if err != nil {
		w.logger.Error("runtime_job_schedule_check_failed", zap.String("job_id", job.ID), zap.Error(err))
		return
	}
	if !accepted {
		w.logger.Info("runtime_job_requeued", zap.String("job_id", job.ID), zap.String("reason", reason), zap.String("worker_id", w.workerID))
		if err := w.stream.Requeue(ctx, job, messageID, 500*time.Millisecond); err != nil {
			w.logger.Error("runtime_job_requeue_failed", zap.String("job_id", job.ID), zap.Error(err))
		}
		return
	}
	if err := w.handle(ctx, job); err != nil {
		w.logger.Error("runtime_job_failed", zap.String("job_id", job.ID), zap.Error(err))
		var failure *operationFailure
		canRetry := errors.As(err, &failure) && failure.retryable && job.Attempts < 3
		if canRetry {
			if recordErr := w.recordRetry(ctx, failure); recordErr != nil {
				w.logger.Error("runtime_job_retry_record_failed", zap.String("job_id", job.ID), zap.Error(recordErr))
				return
			}
			delay := time.Duration(1<<job.Attempts) * time.Second
			if retryErr := w.stream.Retry(ctx, job, messageID, delay); retryErr != nil {
				w.logger.Error("runtime_job_retry_failed", zap.String("job_id", job.ID), zap.Error(retryErr))
			}
			return
		}
		if failure != nil {
			if recordErr := w.recordFailure(ctx, failure); recordErr != nil {
				w.logger.Error("runtime_job_failure_record_failed", zap.String("job_id", job.ID), zap.Error(recordErr))
				return
			}
		}
		if deadErr := w.stream.DeadLetter(ctx, job, messageID, "RUNTIME_RETRIES_EXHAUSTED"); deadErr != nil {
			w.logger.Error("runtime_job_dead_letter_failed", zap.String("job_id", job.ID), zap.Error(deadErr))
		}
		return
	}
	_ = w.stream.Ack(ctx, messageID)
}

func (w *Worker) keepJobLockAlive(ctx context.Context, jobID uuid.UUID) func() {
	lockCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-lockCtx.Done():
				return
			case <-ticker.C:
				result := w.db.WithContext(lockCtx).Model(&models.InstanceRuntimeJob{}).
					Where("id=? AND status='running'", jobID).
					Update("locked_at", time.Now().UTC())
				if result.Error != nil && lockCtx.Err() == nil {
					w.logger.Warn("runtime_job_lock_renewal_failed", zap.String("job_id", jobID.String()), zap.Error(result.Error))
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (w *Worker) handle(ctx context.Context, streamJob queue.Job) error {
	var payload jobPayload
	if err := json.Unmarshal(streamJob.Payload, &payload); err != nil {
		return err
	}
	var job models.InstanceRuntimeJob
	skip := false
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id=?", payload.JobID).Error; err != nil {
			return err
		}
		if job.Status == "completed" || job.Status == "failed" {
			skip = true
			return nil
		}
		now := time.Now().UTC()
		if job.Status == "running" && job.LockedAt != nil && job.LockedAt.After(now.Add(-90*time.Second)) {
			skip = true
			return nil
		}
		if err := tx.Model(&job).Updates(map[string]any{"status": "running", "locked_at": now, "attempts": gorm.Expr("attempts+1")}).Error; err != nil {
			return err
		}
		return tx.Model(&models.RuntimeOperation{}).Where("id=? AND status NOT IN ?", job.OperationID, []string{"completed", "failed", "cancelled"}).Updates(map[string]any{"status": "running", "started_at": gorm.Expr("COALESCE(started_at, ?)", now), "retry_count": gorm.Expr("retry_count+1")}).Error
	})
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	stopJobLockHeartbeat := w.keepJobLockAlive(ctx, job.ID)
	defer stopJobLockHeartbeat()
	var instance models.ChallengeInstance
	if err := w.db.WithContext(ctx).First(&instance, "id=?", job.InstanceID).Error; err != nil {
		return w.fail(job, models.ChallengeInstance{}, "INSTANCE_MISSING", err)
	}
	claimUpdates := map[string]any{"worker_id": w.workerID, "heartbeat_at": time.Now().UTC(), "status_version": gorm.Expr("status_version+1")}
	if job.Operation == "start" && instance.Status == string(instancemodule.Pending) {
		claimUpdates["status"] = string(instancemodule.Pulling)
		instance.Status = string(instancemodule.Pulling)
	}
	claim := w.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Where("id=? AND status_version=? AND operation_id=?", instance.ID, instance.StatusVersion, job.OperationID).Updates(claimUpdates)
	if claim.Error != nil {
		return w.fail(job, instance, "INSTANCE_CLAIM_FAILED", claim.Error)
	}
	if claim.RowsAffected != 1 {
		return w.supersede(ctx, job, "STALE_OPERATION")
	}
	instance.StatusVersion++
	instance.WorkerID = w.workerID
	var runtimeCfg models.ChallengeRuntimeConfig
	if instance.RuntimeRevisionID != nil {
		var revision models.ChallengeRuntimeRevision
		if err := w.db.WithContext(ctx).First(&revision, "id=?", *instance.RuntimeRevisionID).Error; err != nil {
			return w.fail(job, instance, "RUNTIME_REVISION_MISSING", err)
		}
		runtimeCfg = models.ChallengeRuntimeConfig{ChallengeID: instance.ChallengeID, RegistryCredentialID: revision.RegistryCredentialID, ImageRef: revision.ImageRef, ImageDigest: revision.ImageDigest, InternalPort: revision.InternalPort, Protocol: revision.Protocol, FlagFormat: revision.FlagFormat, CPUMilli: revision.CPUMilli, MemoryMB: revision.MemoryMB, PIDsLimit: revision.PIDsLimit, DiskMB: revision.DiskMB, TTLSeconds: revision.TTLSeconds, MaxTTLSeconds: revision.MaxTTLSeconds, ReadOnlyRootFS: revision.ReadOnlyRootFS, EnvironmentTemplate: revision.EnvironmentTemplate, Enabled: true}
	} else if err := w.db.WithContext(ctx).Where("challenge_id=? AND enabled=true", instance.ChallengeID).First(&runtimeCfg).Error; err != nil {
		return w.fail(job, instance, "RUNTIME_CONFIG_MISSING", err)
	}
	flag, err := security.Decrypt(instance.FlagCiphertext, w.encryptionKey)
	if err != nil {
		return w.fail(job, instance, "FLAG_DECRYPT_FAILED", err)
	}
	defer zeroBytes(flag)
	if (job.Operation == "start" || job.Operation == "reset") && instance.HostPort == 0 {
		port, err := w.allocatePort(ctx, instance.ID, job.OperationID, runtimeCfg.Protocol, runtimeCfg.InternalPort)
		if err != nil {
			return w.failPortAllocation(job, instance, err)
		}
		instance.HostPort = port
	}
	environment := map[string]string{}
	_ = json.Unmarshal(runtimeCfg.EnvironmentTemplate, &environment)
	spec := runtimeprovider.InstanceSpec{InstanceID: instance.ID.String(), ChallengeID: instance.ChallengeID.String(), ImageRef: runtimeCfg.ImageRef, ImageDigest: runtimeCfg.ImageDigest, DynamicFlag: string(flag), PublicHost: w.cfg.PublicHost, BindHost: w.cfg.BindHost, BaseDomain: w.cfg.BaseDomain, Protocol: runtimeCfg.Protocol, HostPort: instance.HostPort, InternalPort: runtimeCfg.InternalPort, CPUMilli: runtimeCfg.CPUMilli, MemoryMB: runtimeCfg.MemoryMB, PIDsLimit: runtimeCfg.PIDsLimit, DiskMB: runtimeCfg.DiskMB, ReadOnlyRootFS: runtimeCfg.ReadOnlyRootFS, Environment: environment, Labels: map[string]string{"asamu.owner": instance.OwnerID.String(), "asamu.worker": w.workerID}}
	if runtimeCfg.RegistryCredentialID != nil && (job.Operation == "start" || job.Operation == "reset") {
		lease, leaseErr := w.leaseRegistryCredential(ctx, *runtimeCfg.RegistryCredentialID, instance.ID)
		if leaseErr != nil {
			return w.fail(job, instance, "REGISTRY_CREDENTIAL_LEASE_FAILED", leaseErr)
		}
		spec.RegistryHost, spec.RegistryUsername, spec.RegistryToken = lease.RegistryHost, lease.Username, lease.Token
	}
	old := runtimeprovider.RuntimeInstance{ID: instance.RuntimeID, NetworkID: instance.RuntimeNetworkID, AccessURL: instance.AccessURL, HostPort: instance.HostPort, InternalPort: instance.InternalPort}
	var runtimeResult runtimeprovider.RuntimeInstance
	switch job.Operation {
	case "start":
		runtimeResult, err = w.provider.Start(ctx, spec)
	case "restart":
		err = w.provider.Restart(ctx, instance.RuntimeID)
		runtimeResult = old
		runtimeResult.Status = "running"
	case "stop", "expire":
		err = w.provider.Stop(ctx, instance.RuntimeID)
		runtimeResult = old
		runtimeResult.Status = "stopped"
	case "reset":
		runtimeResult, err = w.provider.Reset(ctx, old, spec)
	default:
		err = fmt.Errorf("unsupported runtime operation %s", job.Operation)
	}
	spec.RegistryToken = ""
	if err != nil {
		var providerErr *runtimeprovider.Error
		if errors.As(err, &providerErr) && providerErr.Code == "HOST_PORT_CONFLICT" && instance.HostPort != 0 {
			if quarantineErr := w.quarantinePort(ctx, instance, job.OperationID); quarantineErr != nil {
				return w.fail(job, instance, "PORT_ALLOCATION_FAILED", quarantineErr)
			}
			instance.HostPort = 0
		}
		return w.failRuntime(job, instance, err)
	}
	now := time.Now().UTC()
	updates := map[string]any{"updated_at": now, "error_code": "", "last_error_code": "", "last_error_message": "", "operation_id": nil, "heartbeat_at": now, "status_version": gorm.Expr("status_version+1"), "version": gorm.Expr("version+1")}
	if job.Operation == "stop" || job.Operation == "expire" {
		updates["status"] = "stopped"
		updates["runtime_id"] = ""
		updates["runtime_network_id"] = ""
		updates["access_url"] = ""
		updates["host_port"] = 0
		updates["stopped_at"] = now
		updates["expires_at"] = nil
	} else {
		updates["status"] = "running"
		if runtimeResult.ID != "" {
			updates["runtime_id"] = runtimeResult.ID
		}
		if runtimeResult.NetworkID != "" {
			updates["runtime_network_id"] = runtimeResult.NetworkID
		}
		if runtimeResult.AccessURL != "" {
			updates["access_url"] = runtimeResult.AccessURL
		}
		if job.Operation == "start" || job.Operation == "reset" {
			started := runtimeResult.StartedAt
			if started.IsZero() {
				started = now
			}
			ttl := time.Duration(runtimeCfg.TTLSeconds) * time.Second
			if ttl <= 0 {
				ttl = w.cfg.DefaultTTL
			}
			updates["started_at"] = started
			updates["expires_at"] = started.Add(ttl)
			updates["stopped_at"] = nil
		}
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.ChallengeInstance{}).Where("id=? AND status_version=? AND operation_id=? AND worker_id=?", instance.ID, instance.StatusVersion, job.OperationID, w.workerID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("runtime operation lost instance ownership")
		}
		resultPayload, _ := json.Marshal(map[string]any{"status": updates["status"]})
		if err := tx.Model(&models.RuntimeOperation{}).Where("id=?", job.OperationID).Updates(map[string]any{"status": "completed", "result": json.RawMessage(resultPayload), "error_code": "", "error_message": "", "completed_at": now}).Error; err != nil {
			return err
		}
		if job.Operation == "stop" || job.Operation == "expire" {
			if err := tx.Model(&models.RuntimePortLease{}).Where("instance_id=? AND status IN ?", instance.ID, []string{"reserved", "active", "releasing"}).Updates(map[string]any{"status": "released", "released_at": now, "expires_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.InstancePort{}).Where("instance_id=? AND released_at IS NULL", instance.ID).Update("released_at", now).Error; err != nil {
				return err
			}
			if err := releaseUsage(tx, instance, runtimeCfg, now); err != nil {
				return err
			}
		}
		if job.Operation == "start" || job.Operation == "reset" {
			if err := tx.Model(&models.RuntimePortLease{}).Where("instance_id=? AND status='reserved'", instance.ID).Updates(map[string]any{"status": "active", "activated_at": now, "renewed_at": now, "expires_at": now.Add(2 * time.Minute)}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.InstanceRuntimeJob{}).Where("id=?", job.ID).Updates(map[string]any{"status": "completed", "finished_at": now, "last_error": ""}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.InstanceOperation{}).Where("id=?", job.OperationID).Updates(map[string]any{"result": "success", "to_status": updates["status"], "finished_at": now}).Error; err != nil {
			return err
		}
		event := map[string]any{"id": uuid.New(), "instance_id": instance.ID, "type": "operation.completed", "provider_status": updates["status"], "payload": json.RawMessage(`{"operation":"` + job.Operation + `"}`), "created_at": now}
		return tx.Table("instance_runtime_events").Create(event).Error
	})
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

type registryLease struct {
	RegistryHost string    `json:"registryHost"`
	Username     string    `json:"username"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (w *Worker) leaseRegistryCredential(ctx context.Context, credentialID, instanceID uuid.UUID) (registryLease, error) {
	body, _ := json.Marshal(map[string]any{"instanceId": instanceID})
	url := strings.TrimRight(w.cfg.WorkerInternalAPIURL, "/") + "/internal/runtime/registry-credentials/" + credentialID.String() + "/lease"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return registryLease{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-ID", w.workerID)
	req.Header.Set("X-Worker-Token", w.cfg.WorkerAPIToken)
	req.Header.Set("User-Agent", "asamu-runtime-worker")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return registryLease{}, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	if err != nil {
		return registryLease{}, err
	}
	var envelope struct {
		Success bool          `json:"success"`
		Data    registryLease `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return registryLease{}, fmt.Errorf("registry lease returned invalid response")
	}
	if response.StatusCode != http.StatusOK || !envelope.Success {
		code := "REGISTRY_LEASE_REJECTED"
		if envelope.Error != nil && envelope.Error.Code != "" {
			code = envelope.Error.Code
		}
		return registryLease{}, fmt.Errorf("registry lease rejected: %s", code)
	}
	if envelope.Data.Token == "" || envelope.Data.RegistryHost == "" || time.Now().UTC().After(envelope.Data.ExpiresAt) {
		return registryLease{}, fmt.Errorf("registry lease is empty or expired")
	}
	return envelope.Data, nil
}

func (w *Worker) allocatePort(ctx context.Context, instanceID, operationID uuid.UUID, runtimeProtocol string, internalPort int) (int, error) {
	port, err := w.allocatePortOnce(ctx, instanceID, operationID, runtimeProtocol, internalPort)
	if !errors.Is(err, errRuntimePortPoolExhausted) {
		return port, err
	}

	// A full database lease table does not always mean that the host is really
	// out of ports. After a VM reboot, reinstall, or manual Docker cleanup,
	// old "running" rows may still own active leases although their containers
	// no longer exist. Reconcile once and retry before exposing PORT_EXHAUSTED
	// to the user.
	w.logger.Warn("runtime_port_pool_recovery_started",
		zap.String("worker_id", w.workerID),
		zap.Int("port_min", w.cfg.PortMin),
		zap.Int("port_max", w.cfg.PortMax),
	)
	if reconcileErr := w.Reconcile(ctx); reconcileErr != nil {
		w.logger.Warn("runtime_port_pool_reconcile_failed", zap.Error(reconcileErr))
		return 0, fmt.Errorf("reconcile runtime state before port recovery: %w", reconcileErr)
	}
	if maintenanceErr := w.MaintainLeases(ctx); maintenanceErr != nil {
		return 0, fmt.Errorf("maintain runtime port leases: %w", maintenanceErr)
	}
	if cleanupErr := w.CleanupOrphans(ctx); cleanupErr != nil {
		w.logger.Warn("runtime_port_pool_orphan_cleanup_failed", zap.Error(cleanupErr))
	}

	port, retryErr := w.allocatePortOnce(ctx, instanceID, operationID, runtimeProtocol, internalPort)
	if retryErr != nil {
		if errors.Is(retryErr, errRuntimePortPoolExhausted) {
			return 0, fmt.Errorf("%w (worker=%s range=%d-%d)", errRuntimePortPoolExhausted, w.workerID, w.cfg.PortMin, w.cfg.PortMax)
		}
		return 0, retryErr
	}
	w.logger.Info("runtime_port_pool_recovered", zap.Int("host_port", port), zap.String("worker_id", w.workerID))
	return port, nil
}

func (w *Worker) allocatePortOnce(ctx context.Context, instanceID, operationID uuid.UUID, runtimeProtocol string, internalPort int) (int, error) {
	protocol := portProtocol(runtimeProtocol)
	var allocated int
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("runtime_port_pool").
			Where("worker_id=? AND protocol=? AND (host_port<? OR host_port>?)", w.workerID, protocol, w.cfg.PortMin, w.cfg.PortMax).
			Update("enabled", false).Error; err != nil {
			return err
		}
		if err := tx.Exec(syncRuntimePortPoolSQL, w.workerID, protocol, w.cfg.PortMin, w.cfg.PortMax).Error; err != nil {
			return err
		}
		row := tx.Raw(`SELECT p.host_port FROM runtime_port_pool p
WHERE p.worker_id=? AND p.protocol=? AND p.enabled=true
AND NOT EXISTS (
  SELECT 1 FROM runtime_port_leases l
  WHERE l.worker_id=p.worker_id AND l.protocol=p.protocol AND l.host_port=p.host_port
    AND (l.status IN ('reserved','active','releasing') OR (l.status='conflict' AND l.expires_at>now()))
)
ORDER BY p.host_port
FOR UPDATE OF p SKIP LOCKED
LIMIT 1`, w.workerID, protocol).Row()
		if err := row.Scan(&allocated); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRuntimePortPoolExhausted
			}
			return fmt.Errorf("select runtime port: %w", err)
		}
		now := time.Now().UTC()
		lease := models.RuntimePortLease{ID: uuid.New(), WorkerID: w.workerID, InstanceID: instanceID, OperationID: &operationID, Protocol: protocol, HostPort: allocated, InternalPort: internalPort, Status: "reserved", LeaseToken: uuid.New(), ReservedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
		if err := tx.Create(&lease).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChallengeInstance{}).Where("id=? AND operation_id=?", instanceID, operationID).Update("host_port", allocated).Error
	})
	return allocated, err
}

func (w *Worker) quarantinePort(ctx context.Context, instance models.ChallengeInstance, operationID uuid.UUID) error {
	now := time.Now().UTC()
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.RuntimePortLease{}).
			Where("instance_id=? AND host_port=? AND status IN ?", instance.ID, instance.HostPort, []string{"reserved", "active", "releasing"}).
			Updates(map[string]any{"status": "conflict", "released_at": now, "expires_at": now.Add(5 * time.Minute), "last_error_code": "HOST_PORT_CONFLICT"}).Error; err != nil {
			return err
		}
		result := tx.Model(&models.ChallengeInstance{}).
			Where("id=? AND operation_id=? AND host_port=?", instance.ID, operationID, instance.HostPort).
			Update("host_port", 0)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("lost instance ownership while quarantining a conflicting port")
		}
		return nil
	})
}

func portProtocol(value string) string {
	if strings.EqualFold(value, "udp") {
		return "udp"
	}
	return "tcp"
}
func (w *Worker) fail(job models.InstanceRuntimeJob, instance models.ChallengeInstance, code string, cause error) error {
	retryable := code == "RUNTIME_OPERATION_FAILED" || code == "INSTANCE_CLAIM_FAILED" || code == "PORT_EXHAUSTED" || code == "PORT_ALLOCATION_FAILED"
	return &operationFailure{code: code, message: code, cause: cause, job: job, instance: instance, retryable: retryable}
}

func (w *Worker) failPortAllocation(job models.InstanceRuntimeJob, instance models.ChallengeInstance, cause error) error {
	if errors.Is(cause, errRuntimePortPoolExhausted) {
		return &operationFailure{
			code:      "PORT_EXHAUSTED",
			message:   fmt.Sprintf("no runtime host ports are available in the configured range %d-%d", w.cfg.PortMin, w.cfg.PortMax),
			cause:     cause,
			job:       job,
			instance:  instance,
			retryable: true,
		}
	}
	return &operationFailure{
		code:      "PORT_ALLOCATION_FAILED",
		message:   "runtime host port allocation is temporarily unavailable",
		cause:     cause,
		job:       job,
		instance:  instance,
		retryable: true,
	}
}

func (w *Worker) failRuntime(job models.InstanceRuntimeJob, instance models.ChallengeInstance, cause error) error {
	code := "RUNTIME_OPERATION_FAILED"
	message := "runtime operation failed"
	retryable := true
	var providerErr *runtimeprovider.Error
	if errors.As(cause, &providerErr) {
		code = providerErr.Code
		message = instancemodule.SanitizeRuntimeError(providerErr)
		retryable = runtimeErrorRetryable(code)
	}
	return &operationFailure{code: code, message: message, cause: cause, job: job, instance: instance, retryable: retryable}
}

func runtimeErrorRetryable(code string) bool {
	switch code {
	case "IMAGE_INSPECT_FAILED", "IMAGE_PULL_FAILED", "CONTAINER_INSPECT_FAILED", "CONTAINER_CREATE_FAILED", "CONTAINER_START_FAILED", "CONTAINER_RECOVERY_FAILED", "NETWORK_INSPECT_FAILED", "NETWORK_CREATE_FAILED", "HOST_PORT_CONFLICT":
		return true
	default:
		return false
	}
}

func (w *Worker) recordRetry(ctx context.Context, failure *operationFailure) error {
	now := time.Now().UTC()
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if failure.instance.ID != uuid.Nil {
			if err := tx.Model(&models.ChallengeInstance{}).Where("id=? AND operation_id=?", failure.instance.ID, failure.job.OperationID).Updates(map[string]any{"last_error_code": failure.code, "last_error_message": failure.message, "heartbeat_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.InstanceRuntimeJob{}).Where("id=?", failure.job.ID).Updates(map[string]any{"status": "queued", "available_at": now, "locked_at": nil, "last_error": failure.code}).Error; err != nil {
			return err
		}
		return tx.Model(&models.RuntimeOperation{}).Where("id=?", failure.job.OperationID).Updates(map[string]any{"status": "retrying", "error_code": failure.code, "error_message": failure.message}).Error
	})
}

func (w *Worker) recordFailure(ctx context.Context, failure *operationFailure) error {
	now := time.Now().UTC()
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if failure.instance.ID != uuid.Nil {
			if err := tx.Model(&models.ChallengeInstance{}).Where("id=? AND operation_id=?", failure.instance.ID, failure.job.OperationID).Updates(map[string]any{"status": "failed", "error_code": failure.code, "last_error_code": failure.code, "last_error_message": failure.message, "operation_id": nil, "host_port": 0, "updated_at": now, "status_version": gorm.Expr("status_version+1"), "version": gorm.Expr("version+1")}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.RuntimePortLease{}).Where("instance_id=? AND status IN ?", failure.instance.ID, []string{"reserved", "active", "releasing"}).Updates(map[string]any{"status": "released", "released_at": now, "expires_at": now, "last_error_code": failure.code}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.InstancePort{}).Where("instance_id=? AND released_at IS NULL", failure.instance.ID).Update("released_at", now).Error; err != nil {
				return err
			}
			var runtimeCfg models.ChallengeRuntimeConfig
			if failure.instance.RuntimeRevisionID != nil {
				var revision models.ChallengeRuntimeRevision
				if err := tx.First(&revision, "id=?", *failure.instance.RuntimeRevisionID).Error; err != nil {
					return err
				}
				runtimeCfg = models.ChallengeRuntimeConfig{CPUMilli: revision.CPUMilli, MemoryMB: revision.MemoryMB}
			} else if err := tx.Where("challenge_id=?", failure.instance.ChallengeID).First(&runtimeCfg).Error; err != nil {
				return err
			}
			if err := releaseUsage(tx, failure.instance, runtimeCfg, now); err != nil {
				return err
			}
		}
		if err := tx.Model(&models.InstanceRuntimeJob{}).Where("id=?", failure.job.ID).Updates(map[string]any{"status": "failed", "finished_at": now, "last_error": failure.code}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RuntimeOperation{}).Where("id=?", failure.job.OperationID).Updates(map[string]any{"status": "failed", "error_code": failure.code, "error_message": failure.message, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&models.InstanceOperation{}).Where("id=?", failure.job.OperationID).Updates(map[string]any{"result": "failed", "error_code": failure.code, "finished_at": now}).Error
	})
}

func releaseUsage(tx *gorm.DB, instance models.ChallengeInstance, runtimeCfg models.ChallengeRuntimeConfig, now time.Time) error {
	runtimeSeconds := int64(0)
	if instance.StartedAt != nil && now.After(*instance.StartedAt) {
		runtimeSeconds = int64(now.Sub(*instance.StartedAt).Seconds())
	}
	return tx.Exec(`UPDATE runtime_usage_counters SET
  active_instances=GREATEST(active_instances-1,0),
  reserved_cpu_milli=GREATEST(reserved_cpu_milli-?,0),
  reserved_memory_mb=GREATEST(reserved_memory_mb-?,0),
  runtime_seconds=runtime_seconds+?,
  updated_at=?
WHERE owner_scope=? AND owner_id=? AND day=?`, runtimeCfg.CPUMilli, runtimeCfg.MemoryMB, runtimeSeconds, now, instance.OwnerScope, instance.OwnerID, instance.CreatedAt.UTC().Format("2006-01-02")).Error
}

func (w *Worker) supersede(ctx context.Context, job models.InstanceRuntimeJob, code string) error {
	now := time.Now().UTC()
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.InstanceRuntimeJob{}).Where("id=?", job.ID).Updates(map[string]any{"status": "failed", "finished_at": now, "last_error": code}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RuntimeOperation{}).Where("id=?", job.OperationID).Updates(map[string]any{"status": "cancelled", "error_code": code, "error_message": code, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&models.InstanceOperation{}).Where("id=?", job.OperationID).Updates(map[string]any{"result": "cancelled", "error_code": code, "finished_at": now}).Error
	})
}
func (w *Worker) PublishOutbox(ctx context.Context) error {
	for count := 0; count < 100; count++ {
		found := false
		err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var event struct {
				ID       uuid.UUID
				Payload  json.RawMessage
				Attempts int
			}
			result := tx.Table("outbox_events").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Select("id,payload,attempts").Where("event_type='instance.operation.queued' AND published_at IS NULL").Order("created_at").Limit(1).Find(&event)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			found = true
			var payload jobPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.JobID == uuid.Nil {
				return tx.Table("outbox_events").Where("id=?", event.ID).Updates(map[string]any{"published_at": time.Now().UTC(), "attempts": gorm.Expr("attempts+1"), "last_error": "INVALID_OUTBOX_PAYLOAD"}).Error
			}
			streamPayload, _ := json.Marshal(payload)
			if _, err := w.stream.Publish(ctx, queue.Job{ID: event.ID.String(), Type: "instance.operation", Payload: streamPayload, CreatedAt: time.Now().UTC()}); err != nil {
				_ = tx.Table("outbox_events").Where("id=?", event.ID).Updates(map[string]any{"attempts": gorm.Expr("attempts+1"), "last_error": "REDIS_PUBLISH_FAILED"}).Error
				return err
			}
			return tx.Table("outbox_events").Where("id=? AND published_at IS NULL", event.ID).Updates(map[string]any{"published_at": time.Now().UTC(), "attempts": gorm.Expr("attempts+1"), "last_error": ""}).Error
		})
		if err != nil {
			return err
		}
		if !found {
			break
		}
	}
	return nil
}
func (w *Worker) ExpireDue(ctx context.Context) error {
	var rows []models.ChallengeInstance
	if err := w.db.WithContext(ctx).Where("status='running' AND expires_at<=?", time.Now().UTC()).Limit(100).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		service := instancemodule.NewService(w.db, w.stream, w.cfg, config.Security{FlagHMACSecret: "unused-but-long-enough-unused", FlagEncryptionKey: w.encryptionKey})
		_, err := service.ExpireInstance(ctx, row.OwnerUserID, row.ID, "expire-"+row.ID.String()+"-"+strconv.FormatInt(row.ExpiresAt.Unix(), 10), "worker-expiry")
		if err != nil {
			w.logger.Warn("expire_enqueue_failed", zap.String("instance_id", row.ID.String()), zap.Error(err))
		}
	}
	return nil
}
func (w *Worker) Reconcile(ctx context.Context) error {
	now := time.Now().UTC()
	// These combinations cannot represent a healthy runtime. They can remain
	// after an interrupted upgrade or a failed manual database repair and must
	// not be kept alive by the next heartbeat.
	if err := w.db.WithContext(ctx).Model(&models.ChallengeInstance{}).
		Where(`worker_id=? AND (
			(status='running' AND runtime_id='')
			OR (status IN ('pulling','creating','starting','restarting','resetting','stopping') AND operation_id IS NULL)
		)`, w.workerID).
		Updates(map[string]any{
			"status":             "interrupted",
			"error_code":         "RUNTIME_STATE_INVALID",
			"last_error_code":    "RUNTIME_STATE_INVALID",
			"last_error_message": "runtime state is missing its container or operation",
			"updated_at":         now,
			"status_version":     gorm.Expr("status_version+1"),
		}).Error; err != nil {
		return err
	}

	var rows []models.ChallengeInstance
	if err := w.db.WithContext(ctx).Where("status='running' AND runtime_id<>'' AND worker_id=?", w.workerID).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		status, err := w.provider.Inspect(ctx, row.RuntimeID)
		missing, inspectErr := runtimeMissing(status, err)
		if inspectErr != nil {
			return fmt.Errorf("inspect runtime %s: %w", row.RuntimeID, inspectErr)
		}
		if missing {
			if err := w.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Where("id=? AND status='running' AND worker_id=? AND status_version=?", row.ID, w.workerID, row.StatusVersion).Updates(map[string]any{"status": "interrupted", "error_code": "RUNTIME_DRIFT", "last_error_code": "RUNTIME_DRIFT", "last_error_message": "runtime container is missing or stopped", "updated_at": time.Now().UTC(), "status_version": gorm.Expr("status_version+1")}).Error; err != nil {
				return err
			}
		} else {
			if err := w.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Where("id=? AND status='running' AND worker_id=?", row.ID, w.workerID).Update("heartbeat_at", time.Now().UTC()).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func runtimeMissing(status runtimeprovider.RuntimeStatus, err error) (bool, error) {
	if err == nil {
		return !status.Running, nil
	}
	var providerErr *runtimeprovider.Error
	if errors.As(err, &providerErr) && providerErr.Code == "CONTAINER_NOT_FOUND" {
		return true, nil
	}
	return false, err
}

func (w *Worker) Heartbeat(ctx context.Context) error {
	now := time.Now().UTC()
	imageCtx, cancelImageList := context.WithTimeout(ctx, heartbeatImageListTimeout)
	cachedImages, imageListErr := w.provider.CachedImages(imageCtx)
	cancelImageList()
	lastErrorCode := ""
	workerStatus := "online"
	if imageListErr != nil {
		cachedImages = []string{}
		lastErrorCode = "IMAGE_LIST_FAILED"
		workerStatus = "offline"
		var providerErr *runtimeprovider.Error
		if errors.As(imageListErr, &providerErr) && providerErr.Code != "" {
			lastErrorCode = providerErr.Code
		}
		w.logger.Warn("runtime_image_list_failed", zap.String("worker_id", w.workerID), zap.String("error_code", lastErrorCode), zap.Error(imageListErr))
	}
	protocolsJSON, _ := json.Marshal(w.cfg.WorkerProtocols)
	imagesJSON, _ := json.Marshal(cachedImages)

	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		usage, err := w.currentUsage(tx)
		if err != nil {
			return err
		}
		node := models.RuntimeWorkerNode{WorkerID: w.workerID, Hostname: w.hostname, Status: workerStatus, Enabled: true, CPUTotalMilli: w.cfg.WorkerCPUMilli, MemoryTotalMB: w.cfg.WorkerMemoryMB, MaxInstances: w.cfg.WorkerMaxInstances, ActiveInstances: usage.ActiveInstances, ReservedCPUMilli: usage.CPUMilli, ReservedMemoryMB: usage.MemoryMB, SupportedProtocols: protocolsJSON, CachedImages: imagesJSON, LastErrorCode: lastErrorCode, LastHeartbeat: now, RegisteredAt: now, UpdatedAt: now, Version: 1}
		updates := map[string]any{"hostname": node.Hostname, "status": workerStatus, "cpu_total_milli": node.CPUTotalMilli, "memory_total_mb": node.MemoryTotalMB, "max_instances": node.MaxInstances, "active_instances": node.ActiveInstances, "reserved_cpu_milli": node.ReservedCPUMilli, "reserved_memory_mb": node.ReservedMemoryMB, "supported_protocols": node.SupportedProtocols, "last_error_code": lastErrorCode, "last_heartbeat": now, "updated_at": now}
		if imageListErr == nil {
			updates["cached_images"] = node.CachedImages
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "worker_id"}}, DoUpdates: clause.Assignments(updates)}).Create(&node).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ChallengeInstance{}).
			Where(`worker_id=? AND (
				(status='running' AND runtime_id<>'')
				OR (status IN ('pulling','creating','starting','restarting','resetting','stopping') AND operation_id IS NOT NULL AND EXISTS (
					SELECT 1 FROM instance_runtime_jobs j
					WHERE j.operation_id=challenge_instances.operation_id AND j.status='running' AND j.locked_at>?
				))
			)`, w.workerID, now.Add(-90*time.Second)).
			Update("heartbeat_at", now).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE runtime_port_leases l
SET renewed_at=?, expires_at=?
FROM challenge_instances i
WHERE l.instance_id=i.id
  AND l.worker_id=?
  AND l.status='active'
  AND i.worker_id=?
	  AND (
	    (i.status='running' AND i.runtime_id<>'')
	    OR (i.status IN ('pulling','creating','starting','restarting','resetting','stopping') AND i.operation_id IS NOT NULL AND EXISTS (
	      SELECT 1 FROM instance_runtime_jobs j
	      WHERE j.operation_id=i.operation_id AND j.status='running' AND j.locked_at>?
	    ))
	  )`,
			now, now.Add(2*time.Minute), w.workerID, w.workerID, now.Add(-90*time.Second)).Error
	})
}

type workerUsage struct {
	ActiveInstances int
	CPUMilli        int
	MemoryMB        int
}

func (w *Worker) currentUsage(db *gorm.DB) (workerUsage, error) {
	var usage workerUsage
	err := db.Table("challenge_instances ci").Select(`COUNT(*) AS active_instances,
COALESCE(SUM(COALESCE(rr.cpu_milli,rc.cpu_milli,0)),0) AS cpu_milli,
COALESCE(SUM(COALESCE(rr.memory_mb,rc.memory_mb,0)),0) AS memory_mb`).
		Joins("LEFT JOIN challenge_runtime_revisions rr ON rr.id=ci.runtime_revision_id").
		Joins("LEFT JOIN challenge_runtime_configs rc ON rc.challenge_id=ci.challenge_id AND ci.runtime_revision_id IS NULL").
		Where("ci.worker_id=? AND ci.status IN ?", w.workerID, []string{"pulling", "creating", "starting", "running", "restarting", "resetting", "stopping"}).Scan(&usage).Error
	return usage, err
}

type scheduledJob struct {
	JobID, InstanceID             uuid.UUID
	Operation, WorkerID, Protocol string
	ImageRef                      string
	CPUMilli, MemoryMB            int
}

func (w *Worker) canProcess(ctx context.Context, streamJob queue.Job) (bool, string, error) {
	var payload jobPayload
	if err := json.Unmarshal(streamJob.Payload, &payload); err != nil {
		return true, "", nil
	}
	var job scheduledJob
	err := w.db.WithContext(ctx).Table("instance_runtime_jobs j").Select(`j.id AS job_id,i.id AS instance_id,j.operation,i.worker_id,
COALESCE(rr.protocol,rc.protocol,'tcp') AS protocol,
COALESCE(rr.image_ref,rc.image_ref,'') AS image_ref,
COALESCE(rr.cpu_milli,rc.cpu_milli,0) AS cpu_milli,
COALESCE(rr.memory_mb,rc.memory_mb,0) AS memory_mb`).
		Joins("JOIN challenge_instances i ON i.id=j.instance_id").
		Joins("LEFT JOIN challenge_runtime_revisions rr ON rr.id=i.runtime_revision_id").
		Joins("LEFT JOIN challenge_runtime_configs rc ON rc.challenge_id=i.challenge_id AND i.runtime_revision_id IS NULL").
		Where("j.id=?", payload.JobID).Take(&job).Error
	if err != nil {
		return false, "job_lookup_failed", err
	}
	if job.WorkerID != "" && job.WorkerID != w.workerID {
		return false, "assigned_to_other_worker", nil
	}
	if job.Operation != "start" || job.WorkerID == w.workerID {
		return true, "", nil
	}
	var node models.RuntimeWorkerNode
	if err := w.db.WithContext(ctx).First(&node, "worker_id=?", w.workerID).Error; err != nil {
		return false, "worker_not_registered", err
	}
	usage, err := w.currentUsage(w.db.WithContext(ctx))
	if err != nil {
		return false, "usage_unavailable", err
	}
	protocols := []string{}
	_ = json.Unmarshal(node.SupportedProtocols, &protocols)
	allowed, reason, err := capacityAllows(node, usage, job, protocols)
	if err != nil || !allowed {
		return allowed, reason, err
	}
	if !containsString(node.CachedImages, job.ImageRef) {
		imageJSON, _ := json.Marshal([]string{job.ImageRef})
		var preferred int64
		if err := w.db.WithContext(ctx).Model(&models.RuntimeWorkerNode{}).
			Where("worker_id<>? AND enabled=true AND draining=false AND status='online' AND last_heartbeat>? AND cached_images @> ?::jsonb", w.workerID, time.Now().UTC().Add(-90*time.Second), string(imageJSON)).
			Count(&preferred).Error; err != nil {
			return false, "image_cache_lookup_failed", err
		}
		if preferred > 0 {
			return false, "image_cached_on_other_worker", nil
		}
	}
	return true, "", nil
}

func containsString(raw json.RawMessage, expected string) bool {
	items := []string{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func capacityAllows(node models.RuntimeWorkerNode, usage workerUsage, job scheduledJob, protocols []string) (bool, string, error) {
	if !node.Enabled || node.Draining || node.Status != "online" || node.LastHeartbeat.Before(time.Now().UTC().Add(-90*time.Second)) {
		return false, "worker_not_accepting_starts", nil
	}
	supported := false
	for _, protocol := range protocols {
		if strings.EqualFold(protocol, job.Protocol) || (strings.EqualFold(protocol, "tcp") && strings.EqualFold(job.Protocol, "https")) {
			supported = true
			break
		}
	}
	if !supported {
		return false, "protocol_not_supported", nil
	}
	if usage.ActiveInstances >= node.MaxInstances {
		return false, "instance_capacity_exhausted", nil
	}
	if usage.CPUMilli+job.CPUMilli > node.CPUTotalMilli || usage.MemoryMB+job.MemoryMB > node.MemoryTotalMB {
		return false, "resource_capacity_exhausted", nil
	}
	return true, "", nil
}

func (w *Worker) markOffline() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.db.WithContext(ctx).Model(&models.RuntimeWorkerNode{}).Where("worker_id=?", w.workerID).Updates(map[string]any{"status": "offline", "updated_at": time.Now().UTC(), "version": gorm.Expr("version+1")}).Error
}

func (w *Worker) MaintainLeases(ctx context.Context) error {
	now := time.Now().UTC()
	staleHeartbeatBefore := now.Add(-3 * time.Minute)
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.RuntimePortLease{}).Where("worker_id=? AND status='reserved' AND expires_at<?", w.workerID, now).Updates(map[string]any{"status": "expired", "released_at": now, "last_error_code": "RESERVATION_EXPIRED"}).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE runtime_port_leases l
SET status='released', released_at=?, expires_at=?, last_error_code='INSTANCE_INACTIVE'
FROM challenge_instances i
WHERE l.instance_id=i.id AND l.worker_id=?
  AND l.status IN ('reserved','active','releasing')
  AND i.status IN ('stopped','expired','failed','interrupted','deleted')`, now, now, w.workerID).Error; err != nil {
			return err
		}
		// Active leases have a two-minute TTL. If a worker was restarted after
		// Docker containers were removed, the old rows can still say "running".
		// Reconcile refreshes healthy instances first; only leases whose
		// instance heartbeat is still stale are expired here.
		if err := tx.Exec(`UPDATE runtime_port_leases l
SET status='expired', released_at=?, expires_at=?, last_error_code='LEASE_HEARTBEAT_EXPIRED'
FROM challenge_instances i
WHERE l.instance_id=i.id AND l.worker_id=?
  AND l.status IN ('active','releasing')
  AND l.expires_at<?
  AND (i.heartbeat_at IS NULL OR i.heartbeat_at<?)`,
			now, now, w.workerID, now, staleHeartbeatBefore).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE challenge_instances i SET host_port=0, updated_at=?
WHERE i.worker_id=? AND i.host_port<>0
  AND (
    i.status IN ('stopped','expired','failed','interrupted','deleted')
    OR (i.heartbeat_at IS NULL OR i.heartbeat_at<?)
  )
  AND NOT EXISTS (
    SELECT 1 FROM runtime_port_leases l
    WHERE l.instance_id=i.id AND l.status IN ('reserved','active','releasing')
  )`, now, w.workerID, staleHeartbeatBefore).Error
	})
}

func (w *Worker) CleanupOrphans(ctx context.Context) error {
	var instanceIDs []string
	if err := w.db.WithContext(ctx).Model(&models.ChallengeInstance{}).Where("worker_id=? AND status IN ?", w.workerID, []string{"pending", "pulling", "creating", "starting", "running", "restarting", "resetting", "stopping"}).Pluck("id", &instanceIDs).Error; err != nil {
		return err
	}
	active := make(map[string]bool, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		active[instanceID] = true
	}
	result, err := w.provider.CleanupOrphans(ctx, active)
	if err != nil {
		return err
	}
	if result.Containers > 0 || result.Networks > 0 {
		w.logger.Info("runtime_orphans_cleaned", zap.Int("containers", result.Containers), zap.Int("networks", result.Networks))
	}
	return nil
}
