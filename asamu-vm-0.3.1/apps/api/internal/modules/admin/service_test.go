package admin

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestAnnouncementValidationRunsBeforeDatabase(t *testing.T) {
	service := New(nil)
	if _, err := service.CreateAnnouncement(context.Background(), uuid.New(), AnnouncementInput{Type: "shell", Title: "valid title", Content: "valid content"}); err == nil {
		t.Fatal("invalid announcement type accepted")
	}
	if _, err := service.CreateAnnouncement(context.Background(), uuid.New(), AnnouncementInput{Type: "platform", Title: "x", Content: "valid content"}); err == nil {
		t.Fatal("short announcement title accepted")
	}
}

func TestCheatCaseValidationRunsBeforeDatabase(t *testing.T) {
	service := New(nil)
	if err := service.ResolveCheatCase(context.Background(), uuid.New(), uuid.New(), "deleted", "reason", ""); err == nil {
		t.Fatal("invalid case status accepted")
	}
	if err := service.ResolveCheatCase(context.Background(), uuid.New(), uuid.New(), "closed", "", ""); err == nil {
		t.Fatal("empty close resolution accepted")
	}
}
