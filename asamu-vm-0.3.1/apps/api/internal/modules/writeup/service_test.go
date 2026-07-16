package writeup

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateMutation(t *testing.T) {
	valid := Mutation{Title: "SQLi 复盘", ChallengeID: uuid.New(), Visibility: "unlisted"}
	if err := validateMutation(valid); err != nil {
		t.Fatalf("valid mutation rejected: %v", err)
	}
	invalid := valid
	invalid.Visibility = "staff-only"
	if err := validateMutation(invalid); err == nil {
		t.Fatal("invalid visibility accepted")
	}
	invalid = valid
	invalid.Title = "   "
	if err := validateMutation(invalid); err == nil {
		t.Fatal("blank title accepted")
	}
}

func TestRenderSanitizesActiveContent(t *testing.T) {
	service := New(nil)
	html, err := service.render("# Safe\n<script>alert(1)</script>\n[bad](javascript:alert(1))")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(html)
	if strings.Contains(lower, "<script") || strings.Contains(lower, "javascript:") {
		t.Fatalf("unsafe HTML survived sanitization: %s", html)
	}
}
