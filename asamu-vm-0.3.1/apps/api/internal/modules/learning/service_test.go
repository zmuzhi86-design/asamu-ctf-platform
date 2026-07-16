package learning

import (
	"context"
	"errors"
	"sync"
	"testing"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm/schema"
)

func TestResponseDTOsIgnoreNestedSlicesDuringDatabaseScan(t *testing.T) {
	if _, err := schema.Parse(&Path{}, &sync.Map{}, schema.NamingStrategy{}); err != nil {
		t.Fatalf("Path must be scannable without treating response-only stages as a relation: %v", err)
	}
	if _, err := schema.Parse(&Stage{}, &sync.Map{}, schema.NamingStrategy{}); err != nil {
		t.Fatalf("Stage must be scannable without treating response-only challenges as a relation: %v", err)
	}
}

func TestSaveRejectsEstimatedMinutesAboveDatabaseLimit(t *testing.T) {
	_, err := New(nil).Save(context.Background(), "", Mutation{Slug: "web-foundation", Title: "Web", DirectionKey: "web", EstimatedMinutes: 100001}, uuid.New())
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "INVALID_ESTIMATED_MINUTES" {
		t.Fatalf("expected INVALID_ESTIMATED_MINUTES, got %v", err)
	}
}

func TestManagedStageIndexHandlesEveryDifficultyBand(t *testing.T) {
	for difficulty, expected := range map[string]int{"入门": 0, "简单": 0, "中等": 1, "困难": 2, "专家": 2, "easy": 0, "hard": 2} {
		if actual := managedStageIndex(difficulty, 3); actual != expected {
			t.Fatalf("managedStageIndex(%q, 3)=%d, want %d", difficulty, actual, expected)
		}
	}
}

func TestSaveRejectsExcessiveStageCountBeforeDatabaseAccess(t *testing.T) {
	stages := make([]StageMutation, 51)
	_, err := New(nil).Save(context.Background(), "", Mutation{Slug: "web-foundation", Title: "Web", DirectionKey: "web", EstimatedMinutes: 60, Stages: stages}, uuid.New())
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "TOO_MANY_LEARNING_STAGES" {
		t.Fatalf("expected TOO_MANY_LEARNING_STAGES, got %v", err)
	}
}
