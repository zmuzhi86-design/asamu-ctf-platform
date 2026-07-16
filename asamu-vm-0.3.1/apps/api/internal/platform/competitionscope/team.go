package competitionscope

import (
	"context"
	"errors"

	"asamu.local/platform/api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrAmbiguousTeam = errors.New("competition roster contains the user in multiple teams")

// RegisteredTeam resolves a team competition participant from the immutable
// roster captured at registration time. Current team membership is deliberately
// not consulted: joining or leaving a team after registration must not change
// who can act for that competition entry.
func RegisteredTeam(ctx context.Context, db *gorm.DB, competitionID, userID uuid.UUID, requestedTeamID *uuid.UUID) (models.CompetitionParticipant, error) {
	query := registeredTeamQuery(db.WithContext(ctx), competitionID, userID)
	if requestedTeamID != nil {
		query = query.Where("cp.team_id=?", *requestedTeamID)
	}
	var participants []models.CompetitionParticipant
	if err := query.Order("registered_at DESC").Limit(2).Find(&participants).Error; err != nil {
		return models.CompetitionParticipant{}, err
	}
	if len(participants) == 0 {
		return models.CompetitionParticipant{}, gorm.ErrRecordNotFound
	}
	if len(participants) > 1 {
		return models.CompetitionParticipant{}, ErrAmbiguousTeam
	}
	return participants[0], nil
}

func registeredTeamQuery(db *gorm.DB, competitionID, userID uuid.UUID) *gorm.DB {
	return db.Table("competition_participants cp").
		Select("cp.*").
		Joins("JOIN competition_roster_members crm ON crm.participant_id=cp.id AND crm.competition_id=cp.competition_id").
		Where("cp.competition_id=? AND crm.user_id=? AND cp.team_id IS NOT NULL AND cp.status='registered'", competitionID, userID)
}
