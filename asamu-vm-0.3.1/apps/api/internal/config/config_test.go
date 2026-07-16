package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadRejectsMissingProductionSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_ACCESS_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing secret error")
	}
}
func TestLoadTestConfig(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("FLAG_ENCRYPTION_KEY_BASE64", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	t.Setenv("RUNTIME_PULL_MISSING_IMAGES", "false")
	t.Setenv("RUNTIME_ALLOWED_IMAGES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Security.FlagEncryptionKey) != 32 {
		t.Fatal("encryption key was not loaded")
	}
	if cfg.Runtime.PullMissingImages {
		t.Fatal("challenge images must not be pulled by default")
	}
	if len(cfg.Runtime.AllowedImages) != 0 {
		t.Fatalf("local image mode should not require a default allowlist, got %#v", cfg.Runtime.AllowedImages)
	}
}
func TestLoadRejectsInvalidMailDriver(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("MAIL_DRIVER", "shell")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid mail driver error")
	}
}

func TestLoadAcceptsSingleRuntimePort(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("RUNTIME_PORT_MIN", "29999")
	t.Setenv("RUNTIME_PORT_MAX", "29999")
	if _, err := Load(); err != nil {
		t.Fatalf("single-port runtime pool should be valid: %v", err)
	}
}

func TestLoadRejectsRuntimePortAboveTCPRange(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("RUNTIME_PORT_MIN", "65535")
	t.Setenv("RUNTIME_PORT_MAX", "65536")
	if _, err := Load(); err == nil {
		t.Fatal("runtime port range above 65535 was accepted")
	}
}
