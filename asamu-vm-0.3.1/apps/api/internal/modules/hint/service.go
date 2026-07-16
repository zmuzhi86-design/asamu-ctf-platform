package hint

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/competitionscope"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Item struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Content  string `json:"content,omitempty"`
	Cost     int    `json:"cost"`
	Unlocked bool   `json:"unlocked"`
}
type scope struct {
	challenge                         models.Challenge
	revision                          models.ChallengeRevision
	competitionID, snapshotID, teamID *uuid.UUID
	ownerScope                        string
	ownerID                           uuid.UUID
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, challengeKey string, competitionID, requestedTeamID *uuid.UUID) ([]Item, error) {
	resolved, err := s.resolve(ctx, userID, challengeKey, competitionID, requestedTeamID, false)
	if err != nil {
		return nil, err
	}
	items, err := parse(resolved.revision.HintsJSON)
	if err != nil {
		return nil, err
	}
	var indexes []int
	query := s.db.WithContext(ctx).Table("hint_unlocks").Where("challenge_revision_id=? AND owner_scope=? AND owner_id=?", resolved.revision.ID, resolved.ownerScope, resolved.ownerID)
	if competitionID == nil {
		query = query.Where("competition_id IS NULL")
	} else {
		query = query.Where("competition_id=?", *competitionID)
	}
	if err := query.Pluck("hint_index", &indexes).Error; err != nil {
		return nil, err
	}
	unlocked := map[int]bool{}
	for _, index := range indexes {
		unlocked[index] = true
	}
	for index := range items {
		items[index].Unlocked = unlocked[index]
		if !items[index].Unlocked {
			items[index].Content = ""
		}
	}
	return items, nil
}

func (s *Service) Unlock(ctx context.Context, userID uuid.UUID, challengeKey string, index int, competitionID, requestedTeamID *uuid.UUID) (Item, error) {
	resolved, err := s.resolve(ctx, userID, challengeKey, competitionID, requestedTeamID, true)
	if err != nil {
		return Item{}, err
	}
	items, err := parse(resolved.revision.HintsJSON)
	if err != nil {
		return Item{}, err
	}
	if index < 0 || index >= len(items) {
		return Item{}, httpx.NewError(http.StatusNotFound, "HINT_NOT_FOUND", "Hint 不存在")
	}
	item := items[index]
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if resolved.competitionID != nil {
			if _, err := competitionscope.ActiveChallenge(ctx, tx, *resolved.competitionID, resolved.challenge.ID, true); err != nil {
				return err
			}
		}
		lockKey := "hint:" + resolved.revision.ID.String() + ":" + resolved.ownerID.String() + ":" + strconv.Itoa(index)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey).Error; err != nil {
			return err
		}
		var existing int64
		q := tx.Table("hint_unlocks").Where("challenge_revision_id=? AND hint_index=? AND owner_scope=? AND owner_id=?", resolved.revision.ID, index, resolved.ownerScope, resolved.ownerID)
		if competitionID == nil {
			q = q.Where("competition_id IS NULL")
		} else {
			q = q.Where("competition_id=?", *competitionID)
		}
		if err := q.Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		unlockID := uuid.New()
		record := map[string]any{"id": unlockID, "challenge_id": resolved.challenge.ID, "challenge_revision_id": resolved.revision.ID, "competition_id": competitionID, "competition_snapshot_id": resolved.snapshotID, "hint_index": index, "owner_scope": resolved.ownerScope, "owner_id": resolved.ownerID, "requested_by": userID, "cost": item.Cost, "unlocked_at": time.Now().UTC()}
		if err := tx.Table("hint_unlocks").Create(record).Error; err != nil {
			return err
		}
		if item.Cost > 0 {
			eventID := uuid.New()
			snapshot, _ := json.Marshal(map[string]any{"challengeRevisionId": resolved.revision.ID, "hintIndex": index, "hintCost": item.Cost})
			event := models.ScoreEvent{ID: eventID, UserID: userID, TeamID: resolved.teamID, CompetitionID: competitionID, ChallengeID: &resolved.challenge.ID, Type: "hint", Delta: -item.Cost, ReferenceType: "hint_unlock", ReferenceID: unlockID, RuleSnapshot: snapshot, CreatedAt: time.Now().UTC()}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			if err := tx.Table("hint_unlocks").Where("id=?", unlockID).Update("score_event_id", eventID).Error; err != nil {
				return err
			}
			if resolved.teamID != nil {
				if err := tx.Table("teams").Where("id=?", *resolved.teamID).Update("score", gorm.Expr("score-?", item.Cost)).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	item.Unlocked = true
	return item, nil
}

func (s *Service) resolve(ctx context.Context, userID uuid.UUID, key string, competitionID, requestedTeamID *uuid.UUID, requireActive bool) (scope, error) {
	var out scope
	out.ownerScope = "user"
	out.ownerID = userID
	out.competitionID = competitionID
	if competitionID == nil {
		if err := s.db.WithContext(ctx).Where("(id::text=? OR slug=?) AND status='published'", key, key).First(&out.challenge).Error; err != nil {
			return out, httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
		}
		if out.challenge.CurrentPublishedRevisionID == nil {
			return out, httpx.NewError(http.StatusConflict, "CHALLENGE_REVISION_REQUIRED", "题目没有已发布版本")
		}
		if err := s.db.WithContext(ctx).First(&out.revision, "id=?", *out.challenge.CurrentPublishedRevisionID).Error; err != nil {
			return out, err
		}
		return out, nil
	}
	var competition models.Competition
	if err := s.db.WithContext(ctx).First(&competition, "id=?", *competitionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
		return out, err
	}
	if competition.CurrentSnapshotID == nil {
		return out, httpx.NewError(http.StatusConflict, "COMPETITION_SNAPSHOT_REQUIRED", "比赛尚未生成内容快照")
	}
	// Competition hints are resolved from the immutable current snapshot, not
	// the live publication state. This keeps already-unlocked content readable
	// for registered participants after the match or after archival.
	if err := s.db.WithContext(ctx).Table("challenges c").
		Joins("JOIN competition_challenge_snapshots ccs ON ccs.challenge_id=c.id").
		Where("ccs.competition_snapshot_id=? AND (c.id::text=? OR c.slug=?)", *competition.CurrentSnapshotID, key, key).
		Select("c.*").Take(&out.challenge).Error; err != nil {
		return out, httpx.NewError(http.StatusNotFound, "COMPETITION_CHALLENGE_NOT_FOUND", "比赛题目不存在")
	}
	if requireActive {
		var err error
		competition, err = competitionscope.ActiveChallenge(ctx, s.db, *competitionID, out.challenge.ID, false)
		if err != nil {
			return out, err
		}
	}
	if competition.Mode == "team" {
		participant, err := competitionscope.RegisteredTeam(ctx, s.db, *competitionID, userID, requestedTeamID)
		if err != nil {
			if errors.Is(err, competitionscope.ErrAmbiguousTeam) {
				return out, httpx.NewError(http.StatusConflict, "TEAM_SCOPE_AMBIGUOUS", "当前比赛存在多个报名战队，请明确指定战队")
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return out, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "不在该比赛的报名阵容中")
			}
			return out, err
		}
		if participant.TeamID == nil {
			return out, httpx.NewError(http.StatusConflict, "TEAM_REQUIRED", "团队赛 Hint 必须绑定战队")
		}
		out.ownerScope = "team"
		out.ownerID = *participant.TeamID
		out.teamID = participant.TeamID
	} else {
		if requestedTeamID != nil {
			return out, httpx.NewError(http.StatusBadRequest, "TEAM_SCOPE_NOT_ALLOWED", "个人赛不能指定战队")
		}
		var participant models.CompetitionParticipant
		if err := s.db.WithContext(ctx).Where("competition_id=? AND user_id=? AND team_id IS NULL AND status='registered'", *competitionID, userID).First(&participant).Error; err != nil {
			return out, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "尚未报名该比赛")
		}
	}
	var row struct{ CompetitionSnapshotID, ChallengeRevisionID uuid.UUID }
	if err := s.db.WithContext(ctx).Table("competition_challenge_snapshots").Select("competition_snapshot_id,challenge_revision_id").Where("competition_snapshot_id=? AND challenge_id=?", competition.CurrentSnapshotID, out.challenge.ID).Take(&row).Error; err != nil {
		return out, httpx.NewError(http.StatusNotFound, "COMPETITION_CHALLENGE_NOT_FOUND", "比赛题目不存在")
	}
	out.snapshotID = &row.CompetitionSnapshotID
	if err := s.db.WithContext(ctx).First(&out.revision, "id=?", row.ChallengeRevisionID).Error; err != nil {
		return out, err
	}
	return out, nil
}

func parse(raw json.RawMessage) ([]Item, error) {
	var rows []struct {
		Title, Content  string
		Cost, SortOrder int
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	items := make([]Item, len(rows))
	for index, row := range rows {
		items[index] = Item{Index: index, Title: row.Title, Content: row.Content, Cost: row.Cost}
	}
	return items, nil
}
