package competitionscope

import (
	"context"
	"errors"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ActiveChallenge validates the shared play window used by submissions,
// instances, and hints. Frozen competitions remain playable; freeze only hides
// live scoring. When lock is true, db must be a transaction and the competition
// row is held FOR SHARE until that transaction commits.
func ActiveChallenge(ctx context.Context, db *gorm.DB, competitionID, challengeID uuid.UUID, lock bool) (models.Competition, error) {
	var competition models.Competition
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := query.First(&competition, "id=?", competitionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return competition, httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
		return competition, err
	}
	now := time.Now().UTC()
	if !playableAt(competition, now) {
		return competition, httpx.NewError(http.StatusConflict, "COMPETITION_NOT_ACTIVE", "比赛当前不可答题")
	}
	var count int64
	if err := db.WithContext(ctx).Model(&models.CompetitionChallenge{}).
		Where("competition_id=? AND challenge_id=? AND (opens_at IS NULL OR opens_at<=?)", competitionID, challengeID, now).
		Count(&count).Error; err != nil {
		return competition, err
	}
	if count == 0 {
		return competition, httpx.NewError(http.StatusForbidden, "CHALLENGE_NOT_OPEN", "该题目尚未开放")
	}
	return competition, nil
}

func playableAt(competition models.Competition, now time.Time) bool {
	return (competition.Status == "running" || competition.Status == "frozen") &&
		!now.Before(competition.StartsAt) && now.Before(competition.EndsAt)
}
