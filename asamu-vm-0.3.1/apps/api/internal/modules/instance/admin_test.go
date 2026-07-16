package instance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestValidateAdminTransitionInput(t *testing.T) {
	valid := AdminTransitionInput{Reason: "比赛环境异常，需要停止", ExpectedVersion: 3, UserAgent: string(make([]byte, 600))}
	if err := validateAdminTransitionInput("stop", &valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if len(valid.UserAgent) != 500 {
		t.Fatalf("user agent was not bounded: %d", len(valid.UserAgent))
	}
	for _, input := range []AdminTransitionInput{{Reason: "短", ExpectedVersion: 1}, {Reason: "理由足够", ExpectedVersion: 0}} {
		if err := validateAdminTransitionInput("reset", &input); err == nil {
			t.Fatal("invalid input must be rejected")
		}
	}
	if err := validateAdminTransitionInput("delete", &AdminTransitionInput{Reason: "理由足够", ExpectedVersion: 1}); err == nil {
		t.Fatal("unknown operation must be rejected")
	}
}

func TestRedactRuntimePayload(t *testing.T) {
	redacted := redactRuntimePayload(json.RawMessage(`{"containerId":"safe","message":"checker saw flag{secret}","env":{"FLAG":"flag{secret}","apiToken":"token","nested":[{"password":"bad"}]}}`))
	var value map[string]any
	if err := json.Unmarshal(redacted, &value); err != nil {
		t.Fatal(err)
	}
	env := value["env"].(map[string]any)
	if env["FLAG"] != "[REDACTED]" || env["apiToken"] != "[REDACTED]" || value["containerId"] != "safe" {
		t.Fatalf("unexpected redaction result: %s", redacted)
	}
	if value["message"] != "checker saw [REDACTED]" {
		t.Fatalf("flag-like string was not redacted: %s", redacted)
	}
}

func TestValidateWorkerDrainInput(t *testing.T) {
	valid := AdminWorkerDrainInput{Reason: "节点维护排空", ExpectedVersion: 2}
	if err := validateWorkerDrainInput("worker-01.example", &valid); err != nil {
		t.Fatalf("valid worker drain rejected: %v", err)
	}
	if err := validateWorkerDrainInput("../worker", &valid); err == nil {
		t.Fatal("unsafe worker id must be rejected")
	}
	invalid := AdminWorkerDrainInput{Reason: "短", ExpectedVersion: 0}
	if err := validateWorkerDrainInput("worker-01", &invalid); err == nil {
		t.Fatal("invalid drain metadata must be rejected")
	}
}

func TestResolveScopeChecksTeamMembershipWithoutCompetition(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=127.0.0.1 user=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	teamID := uuid.New()
	_, err = (&Service{db: db}).resolveScope(context.Background(), uuid.New(), uuid.New(), Scope{TeamID: &teamID}, true)
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "NOT_TEAM_MEMBER" {
		t.Fatalf("team scope without membership must be rejected, got %v", err)
	}
}

func TestIdempotentInstanceMustMatchChallengeOwnerAndCompetition(t *testing.T) {
	userID, teamID, challengeID, competitionID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	scope := Scope{CompetitionID: &competitionID, TeamID: &teamID}
	instance := models.ChallengeInstance{ChallengeID: challengeID, OwnerScope: "team", OwnerID: teamID, CompetitionID: &competitionID}
	if !instanceMatchesScope(instance, userID, challengeID, scope) {
		t.Fatal("matching idempotent instance scope was rejected")
	}
	if instanceMatchesScope(instance, userID, uuid.New(), scope) {
		t.Fatal("idempotency key was accepted across challenges")
	}
	otherTeam := uuid.New()
	if instanceMatchesScope(instance, userID, challengeID, Scope{CompetitionID: &competitionID, TeamID: &otherTeam}) {
		t.Fatal("idempotency key was accepted across team owners")
	}
	otherCompetition := uuid.New()
	if instanceMatchesScope(instance, userID, challengeID, Scope{CompetitionID: &otherCompetition, TeamID: &teamID}) {
		t.Fatal("idempotency key was accepted across competitions")
	}
}

func TestQuotaPIDsFieldsUseExistingDatabaseColumn(t *testing.T) {
	for _, row := range []any{&quotaPolicyRow{}, &quotaOverrideRow{}} {
		parsed, err := schema.Parse(row, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		field := parsed.LookUpField("MaxPIDs")
		if field == nil {
			t.Fatal("MaxPIDs field not found")
		}
		if field.DBName != "max_pids" {
			t.Fatalf("MaxPIDs mapped to %q, want max_pids", field.DBName)
		}
	}
}

func TestNewFlagSupportsStandardAndUUIDFormats(t *testing.T) {
	service := &Service{hmacSecret: []byte("test-hmac-secret"), encryptionKey: bytes.Repeat([]byte{7}, 32)}
	standard, _, _, err := service.newFlag("standard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(standard, "flag{cm_") || !strings.HasSuffix(standard, "}") {
		t.Fatalf("unexpected standard Flag: %q", standard)
	}
	uuidFlag, _, _, err := service.newFlag("uuid")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uuidFlag, "flag{") || !strings.HasSuffix(uuidFlag, "}") {
		t.Fatalf("unexpected UUID Flag: %q", uuidFlag)
	}
	rawUUID := strings.TrimSuffix(strings.TrimPrefix(uuidFlag, "flag{"), "}")
	parsed, err := uuid.Parse(rawUUID)
	if err != nil || parsed.String() != rawUUID {
		t.Fatalf("unexpected UUID inside Flag: %q", uuidFlag)
	}
}

func TestNormalizeAccessURLKeepsWebSchemesAndRemovesSocketSchemes(t *testing.T) {
	tests := map[string]string{
		"tcp://192.168.1.36:20000":  "192.168.1.36:20000",
		"UDP://range.example:20001": "range.example:20001",
		"http://range.example:8080": "http://range.example:8080",
		"range.example:20000":       "range.example:20000",
	}
	for input, expected := range tests {
		if actual := normalizeAccessURL(input); actual != expected {
			t.Fatalf("normalizeAccessURL(%q)=%q, want %q", input, actual, expected)
		}
	}
}
