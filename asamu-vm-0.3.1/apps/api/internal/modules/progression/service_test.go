package progression

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestLevelFor(t *testing.T) {
	tiers := []Tier{{Name: "青铜", MinExperience: 0}, {Name: "白银", MinExperience: 100}, {Name: "黄金", MinExperience: 300}}
	current, next, progress := LevelFor(150, tiers)
	if current.Name != "白银" || next == nil || next.Name != "黄金" {
		t.Fatalf("unexpected tier: %+v %+v", current, next)
	}
	if progress < 0.24 || progress > 0.26 {
		t.Fatalf("unexpected progress %f", progress)
	}
}

func TestRewardTargetsLegacyIndividualSnapshot(t *testing.T) {
	userID := uuid.New()
	payload, err := json.Marshal([]map[string]any{{"rank": 1, "userId": userID}})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := rewardTargets("individual", payload)
	if err != nil {
		t.Fatalf("legacy individual snapshot rejected: %v", err)
	}
	if len(targets) != 1 || targets[0].Type != "user" || targets[0].ID != userID || targets[0].Rank != 1 {
		t.Fatalf("unexpected reward target: %+v", targets)
	}
}

func TestRewardTargetsTeamSnapshot(t *testing.T) {
	teamID := uuid.New()
	payload, err := json.Marshal([]map[string]any{{"rank": 2, "subjectType": "team", "subjectId": teamID, "teamId": teamID, "userId": teamID}})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := rewardTargets("team", payload)
	if err != nil {
		t.Fatalf("team snapshot rejected: %v", err)
	}
	if len(targets) != 1 || targets[0].Type != "team" || targets[0].ID != teamID || targets[0].Rank != 2 {
		t.Fatalf("unexpected team reward target: %+v", targets)
	}
}

func TestRewardTargetsRejectAmbiguousLegacyTeamSnapshot(t *testing.T) {
	payload, err := json.Marshal([]map[string]any{{"rank": 1, "userId": uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rewardTargets("team", payload); err == nil {
		t.Fatal("ambiguous legacy team snapshot must fail instead of writing user medals")
	}
}
