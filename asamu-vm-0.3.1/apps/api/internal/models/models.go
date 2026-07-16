package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                                         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Email                                      string         `gorm:"uniqueIndex" json:"email"`
	Username                                   string         `gorm:"uniqueIndex" json:"username"`
	PasswordHash                               string         `json:"-"`
	Status                                     string         `json:"status"`
	EmailVerifiedAt                            *time.Time     `json:"emailVerifiedAt,omitempty"`
	LastLoginAt                                *time.Time     `json:"lastLoginAt,omitempty"`
	CreatedAt                                  time.Time      `json:"createdAt"`
	UpdatedAt                                  time.Time      `json:"updatedAt"`
	TokenVersion                               int            `gorm:"default:1" json:"-"`
	PasswordChangedAt, PendingEmailRequestedAt *time.Time     `json:"-"`
	MustChangePassword                         bool           `json:"mustChangePassword"`
	PendingEmail                               *string        `json:"pendingEmail,omitempty"`
	DeletedAt                                  gorm.DeletedAt `gorm:"index" json:"-"`
}
type UserProfile struct {
	UserID                                                                           uuid.UUID `gorm:"type:uuid;primaryKey" json:"userId"`
	DisplayName, Bio, OrganizationName, AvatarAssetKey, CharacterAssetKey, Signature string
	Skills                                                                           json.RawMessage `gorm:"type:jsonb"`
	Privacy                                                                          json.RawMessage `gorm:"type:jsonb"`
	UpdatedAt                                                                        time.Time
}
type Role struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key, Name string
}
type Permission struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key, Name string
}
type UserRole struct {
	UserID, RoleID uuid.UUID `gorm:"type:uuid;primaryKey"`
}
type RolePermission struct {
	RoleID, PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
}
type RefreshToken struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;index"`
	TokenHash, FamilyID string
	ExpiresAt           time.Time
	UsedAt, RevokedAt   *time.Time
	ReplacedByID        *uuid.UUID
	CreatedAt           time.Time
	IP, UserAgent       string
}
type UserSession struct {
	ID                    uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                uuid.UUID `gorm:"type:uuid;index"`
	RefreshTokenHash      string
	TokenVersion          int
	IP, UserAgent         string
	CreatedAt, LastSeenAt time.Time
	ExpiresAt             time.Time
	RevokedAt             *time.Time
	RevokeReason          string
}

type Team struct {
	ID                                                      uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Slug                                                    string    `gorm:"uniqueIndex" json:"slug"`
	Name, Slogan, Description, FlagAssetKey, BannerAssetKey string
	Recruiting                                              bool
	CaptainID                                               uuid.UUID `gorm:"type:uuid"`
	MemberLimit                                             int
	Score                                                   int64
	CreatedAt, UpdatedAt                                    time.Time
}
type TeamMember struct {
	TeamID, UserID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role           string
	JoinedAt       time.Time
}
type TeamJoinRequest struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	TeamID, UserID  uuid.UUID `gorm:"type:uuid"`
	Message, Status string
	ReviewedBy      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
}

type ChallengeCategory struct {
	ID                               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Key                              string    `gorm:"uniqueIndex" json:"key"`
	Name, Description, SceneAssetKey string
	SortOrder                        int
	Enabled                          bool
}
type Challenge struct {
	ID                                                                                         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Slug                                                                                       string     `gorm:"uniqueIndex" json:"slug"`
	CategoryID                                                                                 uuid.UUID  `gorm:"type:uuid" json:"categoryId"`
	DirectionID                                                                                *uuid.UUID `gorm:"type:uuid" json:"directionId,omitempty"`
	CurrentPublishedRevisionID                                                                 *uuid.UUID `gorm:"type:uuid" json:"currentPublishedRevisionId,omitempty"`
	Title, Difficulty, Summary, DescriptionMarkdown, AuthorName, Status, Visibility, ScoreMode string
	BaseScore, MinimumScore, MaximumScore                                                      int
	DynamicDecay                                                                               int
	IsDynamic, HasAttachment, HasWriteup                                                       bool
	SolveCount, AttemptCount                                                                   int64
	PublishedAt                                                                                *time.Time
	CreatedAt, UpdatedAt                                                                       time.Time
}
type ChallengeRevision struct {
	ID, ChallengeID, CategoryID                                                        uuid.UUID  `gorm:"type:uuid"`
	DirectionID                                                                        *uuid.UUID `gorm:"type:uuid"`
	Version                                                                            int
	Title, Summary, DescriptionMarkdown, Difficulty, AuthorName, Visibility, ScoreMode string
	BaseScore, MinimumScore, MaximumScore, DynamicDecay                                int
	IsDynamic                                                                          bool
	TagsJSON, HintsJSON, KnowledgePointsJSON                                           json.RawMessage `gorm:"type:jsonb"`
	PublishedBy                                                                        *uuid.UUID      `gorm:"type:uuid"`
	PublishedAt, CreatedAt                                                             time.Time
}
type ChallengeRuntimeRevision struct {
	ID, ChallengeRevisionID              uuid.UUID  `gorm:"type:uuid"`
	RegistryCredentialID                 *uuid.UUID `gorm:"type:uuid"`
	ImageRef, ImageDigest, Protocol      string
	FlagFormat                           string
	InternalPort, CPUMilli, MemoryMB     int
	PIDsLimit                            int `gorm:"column:pids_limit"`
	DiskMB, TTLSeconds, MaxTTLSeconds    int
	ReadOnlyRootFS                       bool
	EnvironmentTemplate, HealthcheckJSON json.RawMessage `gorm:"type:jsonb"`
	CreatedAt                            time.Time
}
type ChallengeTag struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"uniqueIndex"`
}
type ChallengeTagLink struct {
	ChallengeID, TagID uuid.UUID `gorm:"type:uuid;primaryKey"`
}
type ChallengeHint struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	ChallengeID            uuid.UUID `gorm:"type:uuid;index"`
	Title, ContentMarkdown string
	Cost, SortOrder        int
	ReleasedAt             *time.Time
}
type ChallengeFile struct {
	ID                                uuid.UUID `gorm:"type:uuid;primaryKey"`
	ChallengeID                       uuid.UUID `gorm:"type:uuid;index"`
	Name, ObjectKey, MIMEType, SHA256 string
	Size                              int64
	Public                            bool
	CreatedAt                         time.Time
	ArchivedAt                        *time.Time
}
type ChallengeKnowledgePoint struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ChallengeID uuid.UUID `gorm:"type:uuid;index"`
	Name        string
	SortOrder   int
}
type ChallengeFlag struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	ChallengeID  uuid.UUID `gorm:"type:uuid;index"`
	Kind         string
	HMAC         []byte
	RegexPattern string
	Stage        int
	Enabled      bool
	CreatedAt    time.Time
}
type ChallengeRuntimeConfig struct {
	ID                                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ChallengeID                       uuid.UUID  `gorm:"type:uuid;uniqueIndex"`
	RegistryCredentialID              *uuid.UUID `gorm:"type:uuid"`
	ImageRef, ImageDigest             string
	InternalPort                      int
	Protocol                          string
	FlagFormat                        string
	CPUMilli, MemoryMB                int
	PIDsLimit                         int `gorm:"column:pids_limit"`
	DiskMB, TTLSeconds, MaxTTLSeconds int
	ReadOnlyRootFS                    bool
	EnvironmentTemplate               json.RawMessage `gorm:"type:jsonb"`
	Enabled                           bool
	UpdatedAt                         time.Time
}

type ChallengeInstance struct {
	ID                                                                         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ChallengeID                                                                uuid.UUID  `gorm:"type:uuid;index" json:"challengeId"`
	OwnerUserID                                                                uuid.UUID  `gorm:"type:uuid;index" json:"ownerUserId"`
	OwnerTeamID                                                                *uuid.UUID `gorm:"type:uuid" json:"ownerTeamId,omitempty"`
	OwnerScope                                                                 string     `json:"ownerScope"`
	OwnerID                                                                    uuid.UUID  `gorm:"type:uuid;index" json:"ownerId"`
	CompetitionID                                                              *uuid.UUID `gorm:"type:uuid" json:"competitionId,omitempty"`
	RuntimeProvider, RuntimeID, RuntimeNetworkID, Status, AccessURL, ErrorCode string     `json:",omitempty"`
	HostPort, InternalPort, Generation                                         int        `json:",omitempty"`
	FlagHMAC, FlagCiphertext                                                   []byte     `json:"-"`
	StartedAt, ExpiresAt, StoppedAt                                            *time.Time `json:",omitempty"`
	CreatedAt                                                                  time.Time  `json:"createdAt"`
	UpdatedAt                                                                  time.Time  `json:"updatedAt"`
	Version                                                                    int64      `gorm:"default:1" json:"version"`
	StatusVersion                                                              int64      `gorm:"default:1" json:"statusVersion"`
	WorkerID                                                                   string     `json:"workerId,omitempty"`
	OperationID, RuntimeRevisionID                                             *uuid.UUID `gorm:"type:uuid" json:",omitempty"`
	LastErrorCode, LastErrorMessage                                            string     `json:",omitempty"`
	HeartbeatAt                                                                *time.Time `json:"heartbeatAt,omitempty"`
}
type InstanceOperation struct {
	ID                                                                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	InstanceID, ActorID                                                uuid.UUID `gorm:"type:uuid;index"`
	Operation, FromStatus, ToStatus, Result, IdempotencyKey, ErrorCode string
	RequestedAt                                                        time.Time
	FinishedAt                                                         *time.Time
	RequestID                                                          string
}
type InstanceRuntimeJob struct {
	ID                      uuid.UUID `gorm:"type:uuid;primaryKey"`
	InstanceID, OperationID uuid.UUID `gorm:"type:uuid;index"`
	Operation, Status       string
	Payload                 json.RawMessage `gorm:"type:jsonb"`
	Attempts                int
	AvailableAt             time.Time
	LockedAt, FinishedAt    *time.Time
	LastError               string
	CreatedAt               time.Time
}
type RuntimeOperation struct {
	ID                      uuid.UUID `gorm:"type:uuid;primaryKey"`
	IdempotencyKey          string
	InstanceID              uuid.UUID `gorm:"type:uuid;index"`
	OperationType           string
	RequestedBy             *uuid.UUID `gorm:"type:uuid;index"`
	Status                  string
	RetryCount, MaxRetries  int
	Payload, Result         json.RawMessage `gorm:"type:jsonb"`
	ErrorCode, ErrorMessage string
	CreatedAt               time.Time
	StartedAt, CompletedAt  *time.Time
}
type InstancePort struct {
	Port        int       `gorm:"primaryKey"`
	InstanceID  uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	AllocatedAt time.Time
	ReleasedAt  *time.Time
}
type RuntimePortLease struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	WorkerID               string
	InstanceID             uuid.UUID  `gorm:"type:uuid;index"`
	OperationID            *uuid.UUID `gorm:"type:uuid"`
	Protocol               string
	HostPort, InternalPort int
	Status                 string
	LeaseToken             uuid.UUID `gorm:"type:uuid"`
	ReservedAt             time.Time
	ActivatedAt, RenewedAt *time.Time
	ExpiresAt              time.Time
	ReleasedAt             *time.Time
	LastErrorCode          string
}
type RuntimeWorkerNode struct {
	WorkerID                               string          `gorm:"primaryKey" json:"workerId"`
	Hostname                               string          `json:"hostname"`
	Status                                 string          `json:"status"`
	Enabled, Draining                      bool            `json:",omitempty"`
	CPUTotalMilli, MemoryTotalMB           int             `json:",omitempty"`
	MaxInstances, ActiveInstances          int             `json:",omitempty"`
	ReservedCPUMilli, ReservedMemoryMB     int             `json:",omitempty"`
	SupportedProtocols, CachedImages       json.RawMessage `gorm:"type:jsonb" json:",omitempty"`
	LastErrorCode                          string          `json:",omitempty"`
	LastHeartbeat, RegisteredAt, UpdatedAt time.Time       `json:",omitempty"`
	Version                                int64           `json:"version"`
}

type RegistryCredential struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name                 string     `json:"name"`
	RegistryHost         string     `gorm:"uniqueIndex" json:"registryHost"`
	Username             string     `json:"username"`
	EncryptedToken       []byte     `json:"-"`
	TokenFingerprint     string     `json:"-"`
	Enabled              bool       `json:"enabled"`
	CreatedBy            *uuid.UUID `gorm:"type:uuid" json:"createdBy,omitempty"`
	LastUsedAt           *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt, UpdatedAt time.Time
	Version              int64 `json:"version"`
}

type Submission struct {
	ID                                         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                                     uuid.UUID  `gorm:"type:uuid;index" json:"userId"`
	ChallengeID                                uuid.UUID  `gorm:"type:uuid;index" json:"challengeId"`
	TeamID, CompetitionID, InstanceID          *uuid.UUID `gorm:"type:uuid" json:",omitempty"`
	ChallengeRevisionID, CompetitionSnapshotID *uuid.UUID `gorm:"type:uuid" json:",omitempty"`
	Result                                     string     `json:"result"`
	AwardedScore                               int        `json:"awardedScore"`
	CandidateFingerprint                       string     `json:"-"`
	IP, UserAgent, RequestID                   string     `json:"-"`
	DurationMS                                 int
	CreatedAt                                  time.Time `json:"createdAt"`
}
type SolveRecord struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID, ChallengeID   uuid.UUID  `gorm:"type:uuid;index"`
	TeamID, CompetitionID *uuid.UUID `gorm:"type:uuid"`
	SubmissionID          uuid.UUID  `gorm:"type:uuid"`
	Score                 int
	SolvedAt              time.Time
}
type BloodRecord struct {
	ID                                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ChallengeID, UserID, SubmissionID uuid.UUID  `gorm:"type:uuid"`
	CompetitionID                     *uuid.UUID `gorm:"type:uuid"`
	Rank                              int
	Award                             int
	CreatedAt                         time.Time
}
type ScoreEvent struct {
	ID                                 uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID                             uuid.UUID  `gorm:"type:uuid;index"`
	TeamID, CompetitionID, ChallengeID *uuid.UUID `gorm:"type:uuid"`
	Type                               string
	Delta                              int
	ReferenceType                      string
	ReferenceID                        uuid.UUID       `gorm:"type:uuid"`
	RuleSnapshot                       json.RawMessage `gorm:"type:jsonb"`
	ParentEventID                      *uuid.UUID      `gorm:"type:uuid"`
	Reason                             string
	CreatedBy                          *uuid.UUID `gorm:"type:uuid"`
	CreatedAt                          time.Time
}

type Competition struct {
	ID                                                                                                  uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Slug                                                                                                string    `gorm:"uniqueIndex" json:"slug"`
	Name, Summary, DescriptionMarkdown, Mode, Status, ScoringMode, Visibility, BannerAssetKey, ThemeKey string
	RegistrationStartsAt, RegistrationEndsAt, StartsAt, EndsAt                                          time.Time
	FreezeAt                                                                                            *time.Time
	CurrentSnapshotID                                                                                   *uuid.UUID `gorm:"type:uuid"`
	TeamMin, TeamMax                                                                                    int
	CreatedAt, UpdatedAt                                                                                time.Time
}
type CompetitionSnapshot struct {
	ID, CompetitionID                 uuid.UUID `gorm:"type:uuid"`
	Version                           int
	Status                            string
	CompetitionJSON, ScoringRulesJSON json.RawMessage `gorm:"type:jsonb"`
	CreatedBy                         *uuid.UUID      `gorm:"type:uuid"`
	CreatedAt, EffectiveAt            time.Time
}
type CompetitionChallengeSnapshot struct {
	ID, CompetitionSnapshotID, ChallengeID, ChallengeRevisionID uuid.UUID  `gorm:"type:uuid"`
	RuntimeRevisionID                                           *uuid.UUID `gorm:"type:uuid"`
	Score, SortOrder                                            int
	OpensAt                                                     *time.Time
	CreatedAt                                                   time.Time
}
type CompetitionChallenge struct {
	CompetitionID, ChallengeID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Score, SortOrder           int
	OpensAt                    *time.Time
}
type CompetitionParticipant struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompetitionID, UserID uuid.UUID  `gorm:"type:uuid;index"`
	TeamID                *uuid.UUID `gorm:"type:uuid"`
	Status                string
	RegisteredAt          time.Time
	RosterSnapshot        json.RawMessage `gorm:"type:jsonb"`
}
type ScoreboardSnapshot struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	CompetitionID uuid.UUID `gorm:"type:uuid;index"`
	Kind          string
	Payload       json.RawMessage `gorm:"type:jsonb"`
	Frozen        bool
	CreatedAt     time.Time
}

type Writeup struct {
	ID                                                                             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Slug                                                                           string     `gorm:"uniqueIndex" json:"slug"`
	AuthorID, ChallengeID                                                          uuid.UUID  `gorm:"type:uuid;index"`
	CompetitionID                                                                  *uuid.UUID `gorm:"type:uuid"`
	Title, Summary, ContentMarkdown, ContentHTML, Status, Visibility, RejectReason string
	Featured                                                                       bool
	PublishedAt, OpensAt                                                           *time.Time
	Views, Likes                                                                   int64
	CreatedAt, UpdatedAt                                                           time.Time
}
type WriteupComment struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	WriteupID, UserID uuid.UUID `gorm:"type:uuid;index"`
	Content           string
	Status            string
	CreatedAt         time.Time
}

type Notification struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID       `gorm:"type:uuid;index" json:"userId"`
	Type      string          `json:"type,omitempty"`
	Title     string          `json:"title,omitempty"`
	Body      string          `json:"body,omitempty"`
	Link      string          `json:"link,omitempty"`
	Payload   json.RawMessage `gorm:"type:jsonb" json:"payload,omitempty"`
	ReadAt    *time.Time      `json:"readAt,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}
type AuditLog struct {
	ID                                                                    uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	ActorID                                                               *uuid.UUID      `gorm:"type:uuid;index" json:"actorId,omitempty"`
	ActorType, Action, ResourceType, ResourceID, IP, UserAgent, RequestID string          `json:",omitempty"`
	BeforeJSON, AfterJSON                                                 json.RawMessage `gorm:"type:jsonb" json:",omitempty"`
	CreatedAt                                                             time.Time       `json:"createdAt"`
}
