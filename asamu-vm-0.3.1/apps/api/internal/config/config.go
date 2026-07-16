package config

import (
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment   string
	HTTPAddr      string
	PublicBaseURL string
	WebOrigins    []string
	Database      Database
	Redis         Redis
	Security      Security
	Storage       Storage
	Runtime       Runtime
	RateLimit     RateLimit
	Mail          Mail
	DocsEnabled   bool
	CookieSecure  bool
}

type Database struct {
	URL              string
	MaxOpen, MaxIdle int
}
type Redis struct {
	Addr, Password, Stream, ConsumerGroup string
	DB                                    int
}
type Security struct {
	JWTIssuer, JWTAccessSecret, FlagHMACSecret, ConfirmationTokenSecret string
	JWTAccessTTL, RefreshTokenTTL                                       time.Duration
	FlagEncryptionKey, RegistryCredentialEncryptionKey                  []byte
}
type Storage struct {
	Driver, LocalRoot string
}
type Runtime struct {
	Provider, PublicHost, BindHost, BaseDomain, DockerHost, WorkerID     string
	WorkerInternalAPIURL, WorkerAPIToken                                 string
	PortMin, PortMax, WorkerCPUMilli, WorkerMemoryMB, WorkerMaxInstances int
	DefaultTTL, MaxTTL                                                   time.Duration
	AllowedImages, WorkerProtocols                                       []string
	Enabled, PullMissingImages                                           bool
}
type RateLimit struct {
	RPS   float64
	Burst int
}
type Mail struct {
	Driver, From, PublicBaseURL, SMTPHost, SMTPUsername, SMTPPassword string
	SMTPPort                                                          int
	PollInterval                                                      time.Duration
}

func Load() (Config, error) {
	flagKey, err := base64.StdEncoding.DecodeString(value("FLAG_ENCRYPTION_KEY_BASE64", ""))
	if err != nil || len(flagKey) != 32 {
		flagKey = nil
	}
	registryKey, err := base64.StdEncoding.DecodeString(value("REGISTRY_CREDENTIAL_ENCRYPTION_KEY_BASE64", ""))
	if err != nil || len(registryKey) != 32 {
		registryKey = nil
	}
	cfg := Config{
		Environment: value("APP_ENV", "development"), HTTPAddr: value("HTTP_ADDR", ":8787"), PublicBaseURL: value("PUBLIC_BASE_URL", "http://127.0.0.1:8787"), WebOrigins: csv("WEB_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173"),
		Database:  Database{URL: value("DATABASE_URL", ""), MaxOpen: integer("DATABASE_MAX_OPEN", 25), MaxIdle: integer("DATABASE_MAX_IDLE", 5)},
		Redis:     Redis{Addr: value("REDIS_ADDR", "127.0.0.1:6379"), Password: value("REDIS_PASSWORD", ""), DB: integer("REDIS_DB", 0), Stream: value("REDIS_STREAM", "asamu.runtime.jobs"), ConsumerGroup: value("REDIS_CONSUMER_GROUP", "asamu-workers")},
		Security:  Security{JWTIssuer: value("JWT_ISSUER", "asamu-platform"), JWTAccessSecret: value("JWT_ACCESS_SECRET", ""), JWTAccessTTL: duration("JWT_ACCESS_TTL", 15*time.Minute), RefreshTokenTTL: duration("REFRESH_TOKEN_TTL", 30*24*time.Hour), FlagHMACSecret: value("FLAG_HMAC_SECRET", ""), FlagEncryptionKey: flagKey, RegistryCredentialEncryptionKey: registryKey, ConfirmationTokenSecret: value("CONFIRMATION_TOKEN_SECRET", "")},
		Storage:   Storage{Driver: value("STORAGE_DRIVER", "local"), LocalRoot: value("LOCAL_STORAGE_ROOT", "./var/storage")},
		Runtime:   Runtime{Provider: value("RUNTIME_PROVIDER", "docker"), PublicHost: value("RUNTIME_PUBLIC_HOST", "127.0.0.1"), BindHost: value("RUNTIME_BIND_HOST", "127.0.0.1"), BaseDomain: value("RUNTIME_BASE_DOMAIN", "challenge.localhost"), PortMin: integer("RUNTIME_PORT_MIN", 20000), PortMax: integer("RUNTIME_PORT_MAX", 30000), DefaultTTL: duration("RUNTIME_DEFAULT_TTL", 2*time.Hour), MaxTTL: duration("RUNTIME_MAX_TTL", 4*time.Hour), AllowedImages: csv("RUNTIME_ALLOWED_IMAGES", ""), Enabled: boolean("RUNTIME_ENABLED", false), PullMissingImages: boolean("RUNTIME_PULL_MISSING_IMAGES", false), DockerHost: value("RUNTIME_DOCKER_HOST", ""), WorkerID: value("RUNTIME_WORKER_ID", ""), WorkerInternalAPIURL: value("RUNTIME_WORKER_INTERNAL_API_URL", "http://api:8787/api/v1"), WorkerAPIToken: value("RUNTIME_WORKER_API_TOKEN", ""), WorkerCPUMilli: integer("RUNTIME_WORKER_CPU_MILLI", 4000), WorkerMemoryMB: integer("RUNTIME_WORKER_MEMORY_MB", 8192), WorkerMaxInstances: integer("RUNTIME_WORKER_MAX_INSTANCES", 50), WorkerProtocols: csv("RUNTIME_WORKER_PROTOCOLS", "http,tcp,udp")},
		RateLimit: RateLimit{RPS: decimal("RATE_LIMIT_RPS", 15), Burst: integer("RATE_LIMIT_BURST", 30)}, DocsEnabled: boolean("DOCS_ENABLED", true),
		Mail: Mail{Driver: value("MAIL_DRIVER", "log"), From: value("MAIL_FROM", "noreply@asamu.local"), PublicBaseURL: value("MAIL_PUBLIC_BASE_URL", "http://127.0.0.1:5173"), SMTPHost: value("MAIL_SMTP_HOST", ""), SMTPPort: integer("MAIL_SMTP_PORT", 587), SMTPUsername: value("MAIL_SMTP_USERNAME", ""), SMTPPassword: value("MAIL_SMTP_PASSWORD", ""), PollInterval: duration("MAIL_POLL_INTERVAL", 2*time.Second)},
	}
	cfg.CookieSecure = boolean("COOKIE_SECURE", cfg.Environment == "production")
	if cfg.Environment != "test" {
		if cfg.Database.URL == "" {
			return Config{}, errors.New("DATABASE_URL is required")
		}
		if len(cfg.Security.JWTAccessSecret) < 32 || len(cfg.Security.FlagHMACSecret) < 32 || len(cfg.Security.ConfirmationTokenSecret) < 32 || len(cfg.Security.FlagEncryptionKey) != 32 || len(cfg.Security.RegistryCredentialEncryptionKey) != 32 {
			return Config{}, errors.New("security secrets must be configured with at least 32 bytes; encryption keys must decode to 32 bytes")
		}
	}
	if cfg.Runtime.Enabled && len(cfg.Runtime.WorkerAPIToken) < 32 {
		return Config{}, errors.New("RUNTIME_WORKER_API_TOKEN must contain at least 32 bytes when runtime is enabled")
	}
	if cfg.Runtime.PortMin < 1024 || cfg.Runtime.PortMax < cfg.Runtime.PortMin || cfg.Runtime.PortMax > 65535 {
		return Config{}, errors.New("invalid runtime port range")
	}
	if cfg.Runtime.WorkerCPUMilli < 1 || cfg.Runtime.WorkerMemoryMB < 1 || cfg.Runtime.WorkerMaxInstances < 1 {
		return Config{}, errors.New("runtime worker capacity must be positive")
	}
	if cfg.RateLimit.RPS <= 0 || cfg.RateLimit.Burst < 1 {
		return Config{}, errors.New("rate limit RPS and burst must be positive")
	}
	if cfg.Mail.Driver != "log" && cfg.Mail.Driver != "smtp" && cfg.Mail.Driver != "disabled" {
		return Config{}, errors.New("MAIL_DRIVER must be log, smtp, or disabled")
	}
	if cfg.Mail.Driver == "smtp" && (cfg.Mail.SMTPHost == "" || cfg.Mail.From == "" || cfg.Mail.SMTPPort < 1) {
		return Config{}, errors.New("SMTP mail driver requires MAIL_SMTP_HOST, MAIL_SMTP_PORT, and MAIL_FROM")
	}
	return cfg, nil
}

func value(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}
func csv(key, fallback string) []string {
	raw := value(key, fallback)
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func integer(key string, fallback int) int {
	v, err := strconv.Atoi(value(key, ""))
	if err != nil {
		return fallback
	}
	return v
}
func decimal(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(value(key, ""), 64)
	if err != nil {
		return fallback
	}
	return v
}
func boolean(key string, fallback bool) bool {
	v, err := strconv.ParseBool(value(key, ""))
	if err != nil {
		return fallback
	}
	return v
}
func duration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(value(key, ""))
	if err != nil {
		return fallback
	}
	return v
}
