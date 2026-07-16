package scoreboard

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDynamicScore(t *testing.T) {
	rule := DynamicRule{Maximum: 500, Minimum: 100, Decay: 50}
	tests := []struct {
		solves           int64
		wantMin, wantMax int
	}{{0, 500, 500}, {10, 480, 490}, {50, 290, 310}, {100, 170, 190}, {10000, 100, 101}}
	for _, test := range tests {
		got := DynamicScore(rule, test.solves)
		if got < test.wantMin || got > test.wantMax {
			t.Fatalf("solves=%d got=%d outside [%d,%d]", test.solves, got, test.wantMin, test.wantMax)
		}
	}
}
func TestBloodBonusOrder(t *testing.T) {
	first := BloodBonus(1, 500)
	second := BloodBonus(2, 500)
	third := BloodBonus(3, 500)
	if !(first > second && second > third && third > 0) {
		t.Fatalf("unexpected bonuses %d %d %d", first, second, third)
	}
}

func TestPlanPublicView(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	freezeAt := now.Add(-time.Hour)
	updatedAt := now.Add(-30 * time.Minute)

	running := planPublicView(competitionState{Status: "running", FreezeAt: &freezeAt, UpdatedAt: updatedAt}, now)
	if running.SnapshotKind != "" || running.Cutoff == nil || !running.Cutoff.Equal(freezeAt) {
		t.Fatalf("running competition must replay at freeze_at: %+v", running)
	}
	frozen := planPublicView(competitionState{Status: "frozen", FreezeAt: &freezeAt, UpdatedAt: updatedAt}, now)
	if frozen.SnapshotKind != "freeze" || frozen.Cutoff == nil || !frozen.Cutoff.Equal(updatedAt) {
		t.Fatalf("frozen competition must prefer snapshot and replay at updated_at: %+v", frozen)
	}
	finished := planPublicView(competitionState{Status: "finished", UpdatedAt: updatedAt}, now)
	if finished.SnapshotKind != "final" || finished.Cutoff == nil || !finished.Cutoff.Equal(updatedAt) {
		t.Fatalf("finished competition must prefer final settlement: %+v", finished)
	}
	beforeFreeze := now.Add(time.Hour)
	visible := planPublicView(competitionState{Status: "running", FreezeAt: &beforeFreeze}, now)
	if visible.SnapshotKind != "" || visible.Cutoff != nil {
		t.Fatalf("pre-freeze scoreboard must remain live: %+v", visible)
	}
}

func TestNormalizeSnapshotRows(t *testing.T) {
	userID := uuid.New()
	legacy := []Row{{Rank: 1, UserID: userID, Username: "alice"}}
	if err := normalizeSnapshotRows(legacy, "individual"); err != nil {
		t.Fatalf("legacy individual snapshot rejected: %v", err)
	}
	if legacy[0].SubjectType != "user" || legacy[0].SubjectID != userID || legacy[0].UserID != userID {
		t.Fatalf("legacy individual identity was not normalized: %+v", legacy[0])
	}

	teamID := uuid.New()
	team := []Row{{Rank: 1, SubjectType: "team", SubjectID: teamID, TeamName: "red"}}
	if err := normalizeSnapshotRows(team, "team"); err != nil {
		t.Fatalf("team snapshot rejected: %v", err)
	}
	if team[0].TeamID == nil || *team[0].TeamID != teamID || team[0].UserID != teamID || team[0].Username != "red" {
		t.Fatalf("team aliases were not normalized: %+v", team[0])
	}

	ambiguous := []Row{{Rank: 1, UserID: uuid.New(), Username: "registrar"}}
	if err := normalizeSnapshotRows(ambiguous, "team"); err == nil {
		t.Fatal("legacy user snapshot must not be served as a team scoreboard")
	}
}
