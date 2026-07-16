package submission

import (
	"encoding/json"
	"strings"
	"testing"

	"asamu.local/platform/api/internal/models"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSharedFlagDetectionOnlyRunsForCorrectDynamicSubmissions(t *testing.T) {
	tests := []struct {
		name             string
		correct, dynamic bool
		want             bool
	}{
		{name: "correct dynamic flag", correct: true, dynamic: true, want: true},
		{name: "correct static flag", correct: true, dynamic: false},
		{name: "incorrect dynamic flag", correct: false, dynamic: true},
		{name: "incorrect static flag", correct: false, dynamic: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldDetectSharedFlag(test.correct, test.dynamic); got != test.want {
				t.Fatalf("shouldDetectSharedFlag(%v, %v)=%v, want %v", test.correct, test.dynamic, got, test.want)
			}
		})
	}
}

func TestSharedFlagQueryExcludesTheSameTeamInstance(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=test dbname=test sslmode=disable", PreferSimpleProtocol: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	instanceID := uuid.New()
	var submission models.Submission
	result := sharedFlagCandidateQuery(db, uuid.New(), "fingerprint", &instanceID).First(&submission)
	query := result.Statement.SQL.String()
	if !strings.Contains(query, "instance_id IS DISTINCT FROM") {
		t.Fatalf("same shared instance was not excluded: %s", query)
	}
}

func TestCompetitionSnapshotScoringModeOverridesChallengeRevision(t *testing.T) {
	if got := snapshotScoreMode(json.RawMessage(`{"scoringMode":"fixed"}`), "dynamic"); got != "fixed" {
		t.Fatalf("snapshot score mode was ignored: %s", got)
	}
	if got := snapshotScoreMode(json.RawMessage(`{"scoringMode":"custom"}`), "dynamic"); got != "dynamic" {
		t.Fatalf("invalid snapshot score mode should fall back: %s", got)
	}
}

func TestSolveLockUsesTeamAsCompetitionScoringOwner(t *testing.T) {
	userID, teamID, challengeID, competitionID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	teamKey := solveLockKey(userID, &teamID, challengeID, &competitionID)
	if !strings.HasPrefix(teamKey, "team:"+teamID.String()+":") || strings.Contains(teamKey, userID.String()) {
		t.Fatalf("team solve lock has wrong owner: %s", teamKey)
	}
	userKey := solveLockKey(userID, nil, challengeID, &competitionID)
	if !strings.HasPrefix(userKey, "user:"+userID.String()+":") {
		t.Fatalf("individual solve lock has wrong owner: %s", userKey)
	}
}
