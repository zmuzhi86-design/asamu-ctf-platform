package competitionscope

import (
	"testing"
	"time"

	"asamu.local/platform/api/internal/models"
)

func TestPlayableAtAllowsFrozenButHonorsCompetitionWindow(t *testing.T) {
	now := time.Now().UTC()
	base := models.Competition{Status: "running", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)}
	tests := []struct {
		name        string
		competition models.Competition
		at          time.Time
		want        bool
	}{
		{name: "running", competition: base, at: now, want: true},
		{name: "frozen", competition: withStatus(base, "frozen"), at: now, want: true},
		{name: "registration", competition: withStatus(base, "registration"), at: now},
		{name: "before start", competition: base, at: base.StartsAt.Add(-time.Nanosecond)},
		{name: "at end", competition: base, at: base.EndsAt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := playableAt(test.competition, test.at); got != test.want {
				t.Fatalf("playableAt()=%v, want %v", got, test.want)
			}
		})
	}
}

func withStatus(competition models.Competition, status string) models.Competition {
	competition.Status = status
	return competition
}
