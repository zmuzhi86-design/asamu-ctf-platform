package learning

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Challenge struct {
	ID         uuid.UUID `json:"id"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Difficulty string    `json:"difficulty"`
	Score      int       `json:"score"`
	Dynamic    bool      `json:"dynamic"`
	Required   bool      `json:"required"`
	Completed  bool      `json:"completed"`
	SortOrder  int       `json:"sortOrder"`
}

type Stage struct {
	ID                  uuid.UUID   `json:"id"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	SortOrder           int         `json:"sortOrder"`
	Challenges          []Challenge `gorm:"-" json:"challenges"`
	CompletedChallenges int         `json:"completedChallenges"`
	TotalChallenges     int         `json:"totalChallenges"`
	Completed           bool        `json:"completed"`
}

type Path struct {
	ID                  uuid.UUID  `json:"id"`
	Slug                string     `json:"slug"`
	DirectionID         *uuid.UUID `json:"directionId,omitempty"`
	DirectionKey        string     `json:"directionKey"`
	DirectionName       string     `json:"directionName"`
	SceneAssetKey       string     `json:"sceneAssetKey"`
	Title               string     `json:"title"`
	Summary             string     `json:"summary"`
	Description         string     `json:"description"`
	Prerequisite        string     `json:"prerequisite"`
	EstimatedMinutes    int        `json:"estimatedMinutes"`
	HeroAssetKey        string     `json:"heroAssetKey"`
	Status              string     `json:"status"`
	Featured            bool       `json:"featured"`
	SortOrder           int        `json:"sortOrder"`
	PublishedAt         *time.Time `json:"publishedAt,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	Stages              []Stage    `gorm:"-" json:"stages"`
	CompletedChallenges int        `json:"completedChallenges"`
	TotalChallenges     int        `json:"totalChallenges"`
	Progress            float64    `json:"progress"`
}

type StageMutation struct {
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	SortOrder    int         `json:"sortOrder"`
	ChallengeIDs []uuid.UUID `json:"challengeIds"`
}

type Mutation struct {
	Slug             string          `json:"slug"`
	DirectionKey     string          `json:"directionKey"`
	Title            string          `json:"title"`
	Summary          string          `json:"summary"`
	Description      string          `json:"description"`
	Prerequisite     string          `json:"prerequisite"`
	EstimatedMinutes int             `json:"estimatedMinutes"`
	HeroAssetKey     string          `json:"heroAssetKey"`
	Featured         bool            `json:"featured"`
	SortOrder        int             `json:"sortOrder"`
	Stages           []StageMutation `json:"stages"`
}

func (s *Service) List(ctx context.Context, userID *uuid.UUID, admin bool) ([]Path, error) {
	query := s.db.WithContext(ctx).Table("learning_paths lp").Select(`lp.id,lp.slug,lp.direction_id,COALESCE(cc.key,'') AS direction_key,COALESCE(cc.name,'') AS direction_name,COALESCE(cc.scene_asset_key,'') AS scene_asset_key,lp.title,lp.summary,lp.description,lp.prerequisite,lp.estimated_minutes,lp.hero_asset_key,lp.status,lp.featured,lp.sort_order,lp.published_at,lp.updated_at`).Joins("LEFT JOIN challenge_categories cc ON cc.id=lp.direction_id")
	if !admin {
		query = query.Where("lp.status='published'")
	}
	paths := []Path{}
	if err := query.Order("lp.featured DESC,lp.sort_order,lp.title").Scan(&paths).Error; err != nil {
		return nil, err
	}
	visible := make([]Path, 0, len(paths))
	for index := range paths {
		if err := s.loadStages(ctx, &paths[index], userID, admin); err != nil {
			return nil, err
		}
		if admin || paths[index].TotalChallenges > 0 {
			visible = append(visible, paths[index])
		}
	}
	return visible, nil
}

func (s *Service) Detail(ctx context.Context, identifier string, userID *uuid.UUID, admin bool) (Path, error) {
	paths, err := s.List(ctx, userID, admin)
	if err != nil {
		return Path{}, err
	}
	for _, item := range paths {
		if item.ID.String() == identifier || strings.EqualFold(item.Slug, identifier) {
			return item, nil
		}
	}
	return Path{}, httpx.NewError(http.StatusNotFound, "LEARNING_PATH_NOT_FOUND", "训练路线不存在")
}

func (s *Service) loadStages(ctx context.Context, path *Path, userID *uuid.UUID, admin bool) error {
	path.Stages = []Stage{}
	if err := s.db.WithContext(ctx).Table("learning_stages").Select("id,title,description,sort_order").Where("path_id=?", path.ID).Order("sort_order,title").Scan(&path.Stages).Error; err != nil {
		return err
	}
	for stageIndex := range path.Stages {
		stage := &path.Stages[stageIndex]
		stage.Challenges = []Challenge{}
		query := s.db.WithContext(ctx).Table("learning_stage_challenges lsc").Select(`c.id,c.slug,c.title,c.difficulty,c.base_score AS score,c.is_dynamic AS dynamic,lsc.required,lsc.sort_order,false AS completed`).Joins("JOIN challenges c ON c.id=lsc.challenge_id").Where("lsc.stage_id=?", stage.ID)
		if !admin {
			query = query.Where("c.status='published' AND c.visibility='public'")
		}
		if err := query.Order("lsc.sort_order,c.title").Scan(&stage.Challenges).Error; err != nil {
			return err
		}
		if userID != nil && *userID != uuid.Nil && len(stage.Challenges) > 0 {
			ids := make([]uuid.UUID, len(stage.Challenges))
			for index, challenge := range stage.Challenges {
				ids[index] = challenge.ID
			}
			var completed []uuid.UUID
			if err := s.db.WithContext(ctx).Table("submissions").Distinct("challenge_id").Where("user_id=? AND challenge_id IN ? AND result='correct'", *userID, ids).Pluck("challenge_id", &completed).Error; err != nil {
				return err
			}
			done := map[uuid.UUID]bool{}
			for _, id := range completed {
				done[id] = true
			}
			for index := range stage.Challenges {
				stage.Challenges[index].Completed = done[stage.Challenges[index].ID]
			}
		}
		stage.TotalChallenges = len(stage.Challenges)
		for _, challenge := range stage.Challenges {
			if challenge.Completed {
				stage.CompletedChallenges++
			}
		}
		stage.Completed = stage.TotalChallenges > 0 && stage.CompletedChallenges == stage.TotalChallenges
		path.TotalChallenges += stage.TotalChallenges
		path.CompletedChallenges += stage.CompletedChallenges
	}
	if path.TotalChallenges > 0 {
		path.Progress = float64(path.CompletedChallenges) / float64(path.TotalChallenges)
	}
	return nil
}

func (s *Service) Save(ctx context.Context, identifier string, input Mutation, actor uuid.UUID) (Path, error) {
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	input.Title = strings.TrimSpace(input.Title)
	input.DirectionKey = strings.TrimSpace(input.DirectionKey)
	if input.Slug == "" || input.Title == "" || input.DirectionKey == "" {
		return Path{}, httpx.NewError(http.StatusBadRequest, "INVALID_LEARNING_PATH", "路线 Slug、标题和训练方向不能为空")
	}
	if utf8.RuneCountInString(input.Slug) > 128 || utf8.RuneCountInString(input.DirectionKey) > 128 || utf8.RuneCountInString(input.HeroAssetKey) > 128 || utf8.RuneCountInString(input.Title) > 160 {
		return Path{}, httpx.NewError(http.StatusBadRequest, "LEARNING_PATH_TOO_LONG", "路线标识、标题或素材键超过长度限制")
	}
	if len(input.Stages) > 50 {
		return Path{}, httpx.NewError(http.StatusBadRequest, "TOO_MANY_LEARNING_STAGES", "一条路线最多包含 50 个阶段")
	}
	totalChallengeIDs := 0
	for _, stage := range input.Stages {
		if utf8.RuneCountInString(strings.TrimSpace(stage.Title)) > 160 {
			return Path{}, httpx.NewError(http.StatusBadRequest, "STAGE_TITLE_TOO_LONG", "阶段标题不能超过 160 个字符")
		}
		totalChallengeIDs += len(stage.ChallengeIDs)
	}
	if totalChallengeIDs > 500 {
		return Path{}, httpx.NewError(http.StatusBadRequest, "TOO_MANY_STAGE_CHALLENGES", "一条路线最多编排 500 道题")
	}
	if input.EstimatedMinutes < 1 {
		input.EstimatedMinutes = 60
	} else if input.EstimatedMinutes > 100000 {
		return Path{}, httpx.NewError(http.StatusBadRequest, "INVALID_ESTIMATED_MINUTES", "预计学习时长不能超过 100000 分钟")
	}
	var pathID uuid.UUID
	pathStatus := "draft"
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var direction struct{ ID uuid.UUID }
		if err := tx.Table("challenge_categories").Select("id").Where("key=? AND enabled=true", input.DirectionKey).Take(&direction).Error; err != nil {
			return httpx.NewError(http.StatusBadRequest, "INVALID_DIRECTION", "训练方向不存在或已归档")
		}
		now := time.Now().UTC()
		if identifier == "" {
			pathID = uuid.New()
			if err := tx.Table("learning_paths").Create(map[string]any{"id": pathID, "slug": input.Slug, "direction_id": direction.ID, "title": input.Title, "summary": input.Summary, "description": input.Description, "prerequisite": input.Prerequisite, "estimated_minutes": input.EstimatedMinutes, "hero_asset_key": input.HeroAssetKey, "status": "draft", "featured": input.Featured, "sort_order": input.SortOrder, "created_by": actor, "created_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		} else {
			var current struct {
				ID     uuid.UUID
				Status string
			}
			if err := tx.Table("learning_paths").Select("id,status").Where("id::text=? OR slug=?", identifier, identifier).Take(&current).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return httpx.NewError(http.StatusNotFound, "LEARNING_PATH_NOT_FOUND", "训练路线不存在")
				}
				return err
			}
			pathID = current.ID
			status := current.Status
			if status == "archived" {
				status = "draft"
			}
			pathStatus = status
			if err := tx.Table("learning_paths").Where("id=?", pathID).Updates(map[string]any{"slug": input.Slug, "direction_id": direction.ID, "title": input.Title, "summary": input.Summary, "description": input.Description, "prerequisite": input.Prerequisite, "estimated_minutes": input.EstimatedMinutes, "hero_asset_key": input.HeroAssetKey, "status": status, "featured": input.Featured, "sort_order": input.SortOrder, "created_by": actor, "updated_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Table("learning_stages").Where("path_id=?", pathID).Delete(nil).Error; err != nil {
				return err
			}
		}
		assignedChallenges := map[uuid.UUID]bool{}
		for index, stage := range input.Stages {
			stage.Title = strings.TrimSpace(stage.Title)
			if stage.Title == "" {
				return httpx.NewError(http.StatusBadRequest, "STAGE_TITLE_REQUIRED", "阶段标题不能为空")
			}
			stageID := uuid.New()
			sortOrder := stage.SortOrder
			if sortOrder == 0 {
				sortOrder = index + 1
			}
			if err := tx.Table("learning_stages").Create(map[string]any{"id": stageID, "path_id": pathID, "title": stage.Title, "description": stage.Description, "sort_order": sortOrder, "created_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			seen := map[uuid.UUID]bool{}
			for challengeIndex, challengeID := range stage.ChallengeIDs {
				if challengeID == uuid.Nil || seen[challengeID] {
					continue
				}
				seen[challengeID] = true
				if assignedChallenges[challengeID] {
					return httpx.NewError(http.StatusBadRequest, "DUPLICATE_STAGE_CHALLENGE", "同一道题不能重复编排到多个阶段")
				}
				assignedChallenges[challengeID] = true
				var count int64
				if err := tx.Table("challenges").Where("id=? AND status<>'archived'", challengeID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					return httpx.NewError(http.StatusBadRequest, "INVALID_STAGE_CHALLENGE", "阶段包含不存在或已归档的题目")
				}
				if err := tx.Table("learning_stage_challenges").Create(map[string]any{"stage_id": stageID, "challenge_id": challengeID, "sort_order": challengeIndex + 1, "required": true}).Error; err != nil {
					return err
				}
			}
		}
		if pathStatus == "published" {
			var visibleChallengeCount int64
			if err := tx.Table("learning_stage_challenges lsc").Joins("JOIN learning_stages ls ON ls.id=lsc.stage_id").Joins("JOIN challenges c ON c.id=lsc.challenge_id").Where("ls.path_id=? AND c.status='published' AND c.visibility='public'", pathID).Count(&visibleChallengeCount).Error; err != nil {
				return err
			}
			if visibleChallengeCount == 0 {
				return httpx.NewError(http.StatusUnprocessableEntity, "LEARNING_PATH_CONTENT_REQUIRED", "已发布路线至少需要一道公开且已发布的题目")
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Path{}, httpx.NewError(http.StatusConflict, "LEARNING_PATH_SLUG_EXISTS", "路线 Slug 已存在，请换一个")
		}
		return Path{}, err
	}
	return s.Detail(ctx, pathID.String(), nil, true)
}

func (s *Service) Publish(ctx context.Context, identifier string) (Path, error) {
	var pathID uuid.UUID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var path struct{ ID uuid.UUID }
		if err := tx.Table("learning_paths").Select("id").Where("id::text=? OR slug=?", identifier, identifier).Take(&path).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "LEARNING_PATH_NOT_FOUND", "训练路线不存在")
		}
		pathID = path.ID
		var stageCount, challengeCount int64
		if err := tx.Table("learning_stages").Where("path_id=?", pathID).Count(&stageCount).Error; err != nil {
			return err
		}
		if err := tx.Table("learning_stage_challenges lsc").Joins("JOIN learning_stages ls ON ls.id=lsc.stage_id").Joins("JOIN challenges c ON c.id=lsc.challenge_id").Where("ls.path_id=? AND c.status='published' AND c.visibility='public'", pathID).Count(&challengeCount).Error; err != nil {
			return err
		}
		if stageCount == 0 || challengeCount == 0 {
			return httpx.NewError(http.StatusUnprocessableEntity, "LEARNING_PATH_CONTENT_REQUIRED", "发布路线前至少需要一个阶段和一道已发布题目")
		}
		now := time.Now().UTC()
		return tx.Table("learning_paths").Where("id=?", pathID).Updates(map[string]any{"status": "published", "published_at": now, "updated_at": now}).Error
	})
	if err != nil {
		return Path{}, err
	}
	return s.Detail(ctx, pathID.String(), nil, true)
}

func (s *Service) Archive(ctx context.Context, identifier string) error {
	result := s.db.WithContext(ctx).Table("learning_paths").Where("id::text=? OR slug=?", identifier, identifier).Updates(map[string]any{"status": "archived", "featured": false, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return httpx.NewError(http.StatusNotFound, "LEARNING_PATH_NOT_FOUND", "训练路线不存在")
	}
	return nil
}
