package competitionscope

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRegisteredTeamQueryUsesNormalizedFrozenRoster(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=test dbname=test sslmode=disable", PreferSimpleProtocol: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	var participants []struct{ ID uuid.UUID }
	result := registeredTeamQuery(db, uuid.New(), uuid.New()).Find(&participants)
	query := result.Statement.SQL.String()
	if !strings.Contains(query, "JOIN competition_roster_members") || !strings.Contains(query, "crm.user_id") {
		t.Fatalf("query does not authorize through frozen roster: %s", query)
	}
}
