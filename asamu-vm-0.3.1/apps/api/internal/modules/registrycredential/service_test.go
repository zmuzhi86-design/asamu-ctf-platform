package registrycredential

import (
	"encoding/json"
	"strings"
	"testing"

	"asamu.local/platform/api/internal/models"
	"github.com/google/uuid"
)

func TestValidRegistryHost(t *testing.T) {
	for _, value := range []string{"registry.example.com", "registry.example.com:5000", "10.0.0.5:5000"} {
		if !validRegistryHost(value) {
			t.Fatalf("expected valid host %q", value)
		}
	}
	for _, value := range []string{"https://registry.example.com", "registry.example.com/path", "registry.example.com:70000", "user@registry.example.com", "-bad.example"} {
		if validRegistryHost(value) {
			t.Fatalf("expected invalid host %q", value)
		}
	}
}

func TestSecureEqual(t *testing.T) {
	if !secureEqual("same-token", "same-token") {
		t.Fatal("matching tokens should pass")
	}
	if secureEqual("same-token", "different") || secureEqual("", "") {
		t.Fatal("invalid tokens should fail")
	}
}

func TestRegistryHostFromImage(t *testing.T) {
	if got := registryHostFromImage("registry.example.com:5000/team/lab@sha256:abc"); got != "registry.example.com:5000" {
		t.Fatalf("unexpected private registry host %q", got)
	}
	if got := registryHostFromImage("library/nginx@sha256:abc"); got != "docker.io" {
		t.Fatalf("unexpected default registry host %q", got)
	}
	if got := registryHostFromImage("nginx@sha256:abc"); got != "docker.io" {
		t.Fatalf("unexpected single-component registry host %q", got)
	}
}

func TestViewNeverSerializesSecretMaterial(t *testing.T) {
	encoded, err := json.Marshal(view(models.RegistryCredential{ID: uuid.New(), Name: "private", RegistryHost: "registry.example.com", Username: "robot", EncryptedToken: []byte("cipher-secret"), TokenFingerprint: "fingerprint-secret", Enabled: true, Version: 1}))
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	if strings.Contains(value, "cipher-secret") || strings.Contains(value, "fingerprint-secret") || strings.Contains(value, "encryptedToken") || strings.Contains(value, "tokenFingerprint") {
		t.Fatalf("safe DTO leaked secret material: %s", value)
	}
	if !strings.Contains(value, `"tokenConfigured":true`) {
		t.Fatalf("safe DTO should expose only token presence: %s", value)
	}
}
