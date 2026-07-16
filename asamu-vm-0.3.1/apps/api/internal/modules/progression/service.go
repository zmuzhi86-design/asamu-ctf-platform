package progression

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Tier struct {
	ID                 uuid.UUID `json:"id"`
	Key                string    `json:"key"`
	Name               string    `json:"name"`
	BadgeAssetKey      string    `json:"badgeAssetKey"`
	SmallBadgeAssetKey string    `json:"smallBadgeAssetKey"`
	FrameAssetKey      string    `json:"frameAssetKey"`
	Color              string    `json:"color"`
	Gradient           string    `json:"gradient"`
	MinExperience      int64     `json:"minExperience"`
	MaxExperience      *int64    `json:"maxExperience,omitempty"`
	SortOrder          int       `json:"sortOrder"`
}
type Profile struct {
	SchemeID   uuid.UUID `json:"schemeId"`
	SchemeName string    `json:"schemeName"`
	Experience int64     `json:"experience"`
	Tier       Tier      `json:"tier"`
	NextTier   *Tier     `json:"nextTier,omitempty"`
	Progress   float64   `json:"progress"`
	Medals     []Medal   `json:"medals"`
}
type Medal struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Rarity      string    `json:"rarity"`
	AssetKey    string    `json:"assetKey"`
	AwardedAt   time.Time `json:"awardedAt"`
}

func LevelFor(experience int64, tiers []Tier) (Tier, *Tier, float64) {
	if len(tiers) == 0 {
		return Tier{}, nil, 0
	}
	current := tiers[0]
	var next *Tier
	for index, tier := range tiers {
		if experience >= tier.MinExperience {
			current = tier
			if index+1 < len(tiers) {
				copy := tiers[index+1]
				next = &copy
			}
		}
	}
	progress := 1.0
	if next != nil {
		span := next.MinExperience - current.MinExperience
		if span > 0 {
			progress = float64(experience-current.MinExperience) / float64(span)
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}
		}
	}
	return current, next, progress
}
func (s *Service) Profile(ctx context.Context, userID uuid.UUID, schemeKey string) (Profile, error) {
	if schemeKey == "" {
		schemeKey = "platform-default"
	}
	var scheme struct {
		ID   uuid.UUID
		Name string
	}
	if err := s.db.WithContext(ctx).Table("level_schemes").Where("key=? AND enabled=true", schemeKey).First(&scheme).Error; err != nil {
		return Profile{}, httpx.NewError(http.StatusNotFound, "LEVEL_SCHEME_NOT_FOUND", "等级方案不存在")
	}
	var tiers []Tier
	if err := s.db.WithContext(ctx).Table("level_tiers").Where("scheme_id=?", scheme.ID).Order("sort_order").Scan(&tiers).Error; err != nil {
		return Profile{}, err
	}
	var experience int64
	_ = s.db.WithContext(ctx).Table("user_experience").Where("user_id=? AND scheme_id=?", userID, scheme.ID).Pluck("experience", &experience).Error
	current, next, progress := LevelFor(experience, tiers)
	var medals []Medal
	_ = s.db.WithContext(ctx).Table("user_medals um").Select("m.id,m.key,m.name,m.description,m.rarity,m.unlocked_asset_key AS asset_key,um.awarded_at").Joins("JOIN medals m ON m.id=um.medal_id").Where("um.user_id=? AND um.revoked_at IS NULL", userID).Order("um.awarded_at DESC").Scan(&medals).Error
	return Profile{SchemeID: scheme.ID, SchemeName: scheme.Name, Experience: experience, Tier: current, NextTier: next, Progress: progress, Medals: medals}, nil
}
func (s *Service) AddExperience(ctx context.Context, userID uuid.UUID, schemeKey, eventType, referenceType string, referenceID uuid.UUID, delta int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var scheme struct{ ID uuid.UUID }
		if err := tx.Table("level_schemes").Where("key=? AND enabled=true", schemeKey).First(&scheme).Error; err != nil {
			return err
		}
		event := map[string]any{"id": uuid.New(), "user_id": userID, "scheme_id": scheme.ID, "type": eventType, "delta": delta, "reference_type": referenceType, "reference_id": referenceID, "created_at": time.Now().UTC()}
		result := tx.Table("experience_events").Clauses(clause.OnConflict{DoNothing: true}).Create(event)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		record := map[string]any{"user_id": userID, "scheme_id": scheme.ID, "experience": max(delta, 0), "updated_at": time.Now().UTC()}
		return tx.Table("user_experience").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "scheme_id"}}, DoUpdates: clause.Assignments(map[string]any{"experience": gorm.Expr("user_experience.experience+?", delta), "updated_at": time.Now().UTC()})}).Create(record).Error
	})
}
func (s *Service) AwardMedal(ctx context.Context, targetType string, targetID, medalID, sourceID uuid.UUID, sourceType string) error {
	table, column := "user_medals", "user_id"
	if targetType == "team" {
		table, column = "team_medals", "team_id"
	}
	record := map[string]any{"id": uuid.New(), column: targetID, "medal_id": medalID, "source_type": sourceType, "source_id": sourceID, "awarded_at": time.Now().UTC()}
	return s.db.WithContext(ctx).Table(table).Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}
func (s *Service) RevokeMedal(ctx context.Context, targetType string, targetID, medalID uuid.UUID) error {
	table, column := "user_medals", "user_id"
	if targetType == "team" {
		table, column = "team_medals", "team_id"
	}
	return s.db.WithContext(ctx).Table(table).Where(column+"=? AND medal_id=? AND revoked_at IS NULL", targetID, medalID).Update("revoked_at", time.Now().UTC()).Error
}

type rewardRanking struct {
	Rank        int        `json:"rank"`
	SubjectType string     `json:"subjectType"`
	SubjectID   uuid.UUID  `json:"subjectId"`
	UserID      uuid.UUID  `json:"userId"`
	TeamID      *uuid.UUID `json:"teamId"`
}

type rewardTarget struct {
	Rank int
	Type string
	ID   uuid.UUID
}

func rewardTargets(mode string, payload []byte) ([]rewardTarget, error) {
	var rankings []rewardRanking
	if err := json.Unmarshal(payload, &rankings); err != nil {
		return nil, fmt.Errorf("decode scoreboard snapshot: %w", err)
	}
	targets := make([]rewardTarget, 0, len(rankings))
	for _, ranking := range rankings {
		if mode == "team" {
			if ranking.SubjectType == "" {
				return nil, fmt.Errorf("legacy team scoreboard snapshot has ambiguous user subjects; rebuild the final snapshot before distributing rewards")
			}
			if ranking.SubjectType != "team" {
				return nil, fmt.Errorf("team competition snapshot contains %q subject", ranking.SubjectType)
			}
			targetID := ranking.SubjectID
			if targetID == uuid.Nil && ranking.TeamID != nil {
				targetID = *ranking.TeamID
			}
			if targetID == uuid.Nil {
				return nil, fmt.Errorf("team competition snapshot contains an empty team subject")
			}
			if ranking.TeamID != nil && *ranking.TeamID != targetID {
				return nil, fmt.Errorf("team competition snapshot contains inconsistent team subjects")
			}
			targets = append(targets, rewardTarget{Rank: ranking.Rank, Type: "team", ID: targetID})
			continue
		}

		if ranking.SubjectType != "" && ranking.SubjectType != "user" {
			return nil, fmt.Errorf("individual competition snapshot contains %q subject", ranking.SubjectType)
		}
		targetID := ranking.SubjectID
		if targetID == uuid.Nil {
			// Snapshots created before subjectType/subjectId were introduced only
			// carried userId and remain valid for individual competitions.
			targetID = ranking.UserID
		}
		if targetID == uuid.Nil {
			return nil, fmt.Errorf("individual competition snapshot contains an empty user subject")
		}
		if ranking.UserID != uuid.Nil && ranking.UserID != targetID {
			return nil, fmt.Errorf("individual competition snapshot contains inconsistent user subjects")
		}
		targets = append(targets, rewardTarget{Rank: ranking.Rank, Type: "user", ID: targetID})
	}
	return targets, nil
}

func (s *Service) DistributeCompetitionRewards(ctx context.Context, competitionID, snapshotID, actorID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		runID := uuid.New()
		result := tx.Table("reward_distribution_runs").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"id": runID, "competition_id": competitionID, "snapshot_id": snapshotID, "status": "running", "started_by": actorID, "started_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		var competition struct{ Mode string }
		if err := tx.Table("competitions").Select("mode").Where("id=?", competitionID).Take(&competition).Error; err != nil {
			return err
		}
		var snapshot struct{ Payload []byte }
		if err := tx.Table("scoreboard_snapshots").Where("id=? AND competition_id=?", snapshotID, competitionID).First(&snapshot).Error; err != nil {
			return err
		}
		rankings, err := rewardTargets(competition.Mode, snapshot.Payload)
		if err != nil {
			return err
		}
		var rewards []struct {
			ID               uuid.UUID
			RewardType       string
			RankFrom, RankTo *int
			Config           []byte
		}
		if err := tx.Table("competition_rewards").Where("competition_id=? AND enabled=true", competitionID).Find(&rewards).Error; err != nil {
			return err
		}
		for _, reward := range rewards {
			for _, ranking := range rankings {
				if reward.RankFrom != nil && ranking.Rank < *reward.RankFrom {
					continue
				}
				if reward.RankTo != nil && ranking.Rank > *reward.RankTo {
					continue
				}
				var config map[string]string
				_ = json.Unmarshal(reward.Config, &config)
				if reward.RewardType == "medal" {
					medalID, err := uuid.Parse(config["medalId"])
					if err != nil {
						continue
					}
					table, column := "user_medals", "user_id"
					if ranking.Type == "team" {
						table, column = "team_medals", "team_id"
					}
					record := map[string]any{"id": uuid.New(), column: ranking.ID, "medal_id": medalID, "source_type": "competition", "source_id": competitionID, "awarded_at": time.Now().UTC()}
					if err := tx.Table(table).Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
						return err
					}
				}
			}
		}
		return tx.Table("reward_distribution_runs").Where("id=?", runID).Updates(map[string]any{"status": "completed", "finished_at": time.Now().UTC()}).Error
	})
}
func (s *Service) CreateScheme(ctx context.Context, key, name string, tiers []Tier) error {
	if len(tiers) == 0 {
		return httpx.NewError(http.StatusBadRequest, "TIERS_REQUIRED", "等级方案至少包含一个等级")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		schemeID := uuid.New()
		if err := tx.Table("level_schemes").Create(map[string]any{"id": schemeID, "key": key, "name": name, "scope_type": "platform", "enabled": true, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		for _, tier := range tiers {
			tier.ID = uuid.New()
			record := map[string]any{"id": tier.ID, "scheme_id": schemeID, "key": tier.Key, "name": tier.Name, "min_experience": tier.MinExperience, "max_experience": tier.MaxExperience, "sort_order": tier.SortOrder, "badge_asset_key": tier.BadgeAssetKey, "small_badge_asset_key": tier.SmallBadgeAssetKey, "frame_asset_key": tier.FrameAssetKey, "color": tier.Color, "gradient": tier.Gradient}
			if err := tx.Table("level_tiers").Create(record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
