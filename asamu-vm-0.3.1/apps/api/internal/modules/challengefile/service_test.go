package challengefile

import (
	"strings"
	"testing"
)

func TestSafeNameRejectsTraversalAndControls(t *testing.T) {
	for _, value := range []string{"../flag.zip", `folder\flag.zip`, "flag\n.zip", ""} {
		if _, err := safeName(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	if value, err := safeName("challenge-pack.zip"); err != nil || value != "challenge-pack.zip" {
		t.Fatalf("expected safe name, got %q, %v", value, err)
	}
}

func TestBlockedBrowserExecutableTypes(t *testing.T) {
	for _, test := range []struct{ mime, extension string }{{"text/html; charset=utf-8", ".txt"}, {"image/svg+xml", ".svg"}, {"application/octet-stream", ".js"}} {
		if !blockedType(test.mime, test.extension) {
			t.Fatalf("expected %s %s to be blocked", test.mime, test.extension)
		}
	}
	if blockedType("application/zip", ".zip") {
		t.Fatal("zip challenge package should be allowed")
	}
}

func TestDispositionForUnicodeName(t *testing.T) {
	value := Disposition("题目附件.zip")
	if !strings.HasPrefix(value, "attachment") || !strings.Contains(value, "filename") {
		t.Fatalf("unexpected disposition: %s", value)
	}
}
