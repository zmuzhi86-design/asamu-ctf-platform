package team

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestStableSlugFallsBackForNonASCIIName(t *testing.T) {
	id := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	if got := stableSlug("镜像战队", id); got != "team-12345678" {
		t.Fatalf("unexpected fallback slug: %s", got)
	}
	if got := stableSlug("Web Masters", id); got != "web-masters" {
		t.Fatalf("unexpected regular slug: %s", got)
	}
	if !strings.HasPrefix(stableSlug("红队", uuid.New()), "team-") {
		t.Fatal("fallback slug prefix missing")
	}
}
