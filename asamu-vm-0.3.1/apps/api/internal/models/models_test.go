package models

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestPIDsLimitUsesExistingDatabaseColumn(t *testing.T) {
	for _, model := range []any{&ChallengeRuntimeConfig{}, &ChallengeRuntimeRevision{}} {
		parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		field := parsed.LookUpField("PIDsLimit")
		if field == nil {
			t.Fatal("PIDsLimit field not found")
		}
		if field.DBName != "pids_limit" {
			t.Fatalf("PIDsLimit mapped to %q, want pids_limit", field.DBName)
		}
	}
}
