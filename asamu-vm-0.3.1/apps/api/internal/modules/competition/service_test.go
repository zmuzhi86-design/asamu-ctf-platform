package competition

import (
	"testing"

	"asamu.local/platform/api/internal/models"
)

func TestRosterConfigChangeDetection(t *testing.T) {
	current := models.Competition{Mode: "team", TeamMin: 2, TeamMax: 5}
	if rosterConfigChanged(current, Mutation{Mode: "team", TeamMin: 2, TeamMax: 5}) {
		t.Fatal("unchanged roster contract was reported as changed")
	}
	for _, mutation := range []Mutation{
		{Mode: "individual", TeamMin: 2, TeamMax: 5},
		{Mode: "team", TeamMin: 3, TeamMax: 5},
		{Mode: "team", TeamMin: 2, TeamMax: 6},
	} {
		if !rosterConfigChanged(current, mutation) {
			t.Fatalf("roster contract mutation was missed: %+v", mutation)
		}
	}
}
