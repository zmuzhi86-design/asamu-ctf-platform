package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/security"
)

func TestEmailTokenIsOpaqueAndPayloadEncrypted(t *testing.T) {
	secret := strings.Repeat("s", 32)
	svc := NewService(nil, config.Security{JWTAccessSecret: strings.Repeat("j", 32), ConfirmationTokenSecret: secret, JWTAccessTTL: time.Minute}, "https://ctf.example/")
	raw, hash, ciphertext, err := svc.emailToken("player@example.com", "reset_password", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if raw == hash || security.TokenHash(raw) != hash {
		t.Fatal("token must only be persisted as its digest")
	}
	if strings.Contains(string(ciphertext), raw) {
		t.Fatal("outbox payload must be encrypted at rest")
	}
	plain, err := security.Decrypt(ciphertext, svc.confirmationKey)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), "https://ctf.example/reset-password?token="+raw) {
		t.Fatal("reset URL missing from encrypted payload")
	}
}

func TestRegistrationFeatureDefaultsOpenAndHonorsPublishedSnapshot(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "enabled", raw: `{"features":{"registration":true}}`, want: true},
		{name: "disabled", raw: `{"features":{"registration":false}}`},
		{name: "legacy snapshot without flag", raw: `{"features":{"teams":true}}`, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := registrationEnabledFromSnapshot(json.RawMessage(test.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("registrationEnabledFromSnapshot()=%v, want %v", got, test.want)
			}
		})
	}
	if _, err := registrationEnabledFromSnapshot(json.RawMessage(`{"features":`)); err == nil {
		t.Fatal("malformed published snapshot must fail closed")
	}
}

func TestRefreshTokenReplayCandidatesAreDetectedBeforeSessionValidation(t *testing.T) {
	now := time.Now().UTC()
	usedAt, revokedAt := now.Add(-time.Minute), now.Add(-30*time.Second)
	tests := []struct {
		name  string
		token models.RefreshToken
		want  bool
	}{
		{name: "active", token: models.RefreshToken{ExpiresAt: now.Add(time.Hour)}},
		{name: "used", token: models.RefreshToken{UsedAt: &usedAt, ExpiresAt: now.Add(time.Hour)}, want: true},
		{name: "revoked", token: models.RefreshToken{RevokedAt: &revokedAt, ExpiresAt: now.Add(time.Hour)}, want: true},
		{name: "expired", token: models.RefreshToken{ExpiresAt: now}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := refreshTokenNeedsReplayRevocation(test.token, now); got != test.want {
				t.Fatalf("refreshTokenNeedsReplayRevocation()=%v, want %v", got, test.want)
			}
		})
	}
}
