package learning

import (
	"errors"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncPublishedChallenge keeps only system-managed foundation paths in sync.
// A path becomes administrator-managed as soon as created_by is set and is then
// deliberately left untouched by this reconciliation path.
func SyncPublishedChallenge(tx *gorm.DB, challenge models.Challenge, now time.Time) error {
	if err := RemoveManagedChallenge(tx, challenge.ID, now); err != nil {
		return err
	}
	if challenge.Visibility != "public" {
		return nil
	}
	var category models.ChallengeCategory
	if err := tx.Where("id=? AND enabled=true", challenge.CategoryID).Take(&category).Error; err != nil {
		return err
	}
	slug := category.Key + "-foundation"
	pathID := uuid.New()
	if err := tx.Table("learning_paths").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(map[string]any{
		"id": pathID, "slug": slug, "direction_id": category.ID, "title": category.Name + " 安全训练路线",
		"summary":      "从基础知识到综合实战，循序完成 " + category.Name + " 方向训练。",
		"description":  "按照阶段顺序完成已发布题目，解题进度会自动同步到学习中心。",
		"prerequisite": "Linux 基础 / 网络基础", "estimated_minutes": 720, "hero_asset_key": category.SceneAssetKey,
		"status": "draft", "featured": category.Key == "web", "sort_order": category.SortOrder, "created_at": now, "updated_at": now,
	}).Error; err != nil {
		return err
	}
	var path struct {
		ID     uuid.UUID
		Status string
	}
	if err := tx.Table("learning_paths").Select("id,status").Where("slug=? AND created_by IS NULL", slug).Take(&path).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if path.Status == "archived" {
		return nil
	}
	var stages []struct {
		ID        uuid.UUID
		SortOrder int
	}
	if err := tx.Table("learning_stages").Select("id,sort_order").Where("path_id=?", path.ID).Order("sort_order,title").Scan(&stages).Error; err != nil {
		return err
	}
	if len(stages) == 0 {
		for index, title := range []string{"基础入门", "核心技能", "综合实战"} {
			stage := struct {
				ID        uuid.UUID
				SortOrder int
			}{ID: uuid.New(), SortOrder: index + 1}
			if err := tx.Table("learning_stages").Create(map[string]any{"id": stage.ID, "path_id": path.ID, "title": title, "description": "完成本阶段编排的 " + category.Name + " 题目。", "sort_order": stage.SortOrder, "created_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			stages = append(stages, stage)
		}
	}
	stage := stages[managedStageIndex(challenge.Difficulty, len(stages))]
	var sortOrder int
	if err := tx.Table("learning_stage_challenges").Where("stage_id=?", stage.ID).Select("COALESCE(MAX(sort_order),0)+1").Scan(&sortOrder).Error; err != nil {
		return err
	}
	if err := tx.Table("learning_stage_challenges").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"stage_id": stage.ID, "challenge_id": challenge.ID, "sort_order": sortOrder, "required": true}).Error; err != nil {
		return err
	}
	return tx.Table("learning_paths").Where("id=? AND created_by IS NULL AND status<>'archived'", path.ID).Updates(map[string]any{"status": "published", "published_at": gorm.Expr("COALESCE(published_at, ?)", now), "updated_at": now}).Error
}

// RemoveManagedChallenge removes a challenge only from system-managed paths.
func RemoveManagedChallenge(tx *gorm.DB, challengeID uuid.UUID, now time.Time) error {
	var pathIDs []uuid.UUID
	if err := tx.Table("learning_stage_challenges lsc").Distinct("lp.id").Joins("JOIN learning_stages ls ON ls.id=lsc.stage_id").Joins("JOIN learning_paths lp ON lp.id=ls.path_id").Where("lsc.challenge_id=? AND lp.created_by IS NULL", challengeID).Pluck("lp.id", &pathIDs).Error; err != nil {
		return err
	}
	if len(pathIDs) == 0 {
		return nil
	}
	if err := tx.Exec(`DELETE FROM learning_stage_challenges lsc USING learning_stages ls, learning_paths lp
WHERE lsc.stage_id=ls.id AND ls.path_id=lp.id AND lsc.challenge_id=? AND lp.created_by IS NULL`, challengeID).Error; err != nil {
		return err
	}
	for _, pathID := range pathIDs {
		var remaining int64
		if err := tx.Table("learning_stage_challenges lsc").Joins("JOIN learning_stages ls ON ls.id=lsc.stage_id").Joins("JOIN challenges c ON c.id=lsc.challenge_id").Where("ls.path_id=? AND c.status='published' AND c.visibility='public'", pathID).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining == 0 {
			if err := tx.Table("learning_paths").Where("id=? AND created_by IS NULL AND status<>'archived'", pathID).Updates(map[string]any{"status": "draft", "featured": false, "published_at": nil, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func managedStageIndex(difficulty string, stageCount int) int {
	if stageCount <= 1 {
		return 0
	}
	value := strings.ToLower(strings.TrimSpace(difficulty))
	index := 1
	switch value {
	case "入门", "简单", "beginner", "easy", "simple":
		index = 0
	case "困难", "专家", "hard", "expert":
		index = stageCount - 1
	}
	if index >= stageCount {
		return stageCount - 1
	}
	return index
}
