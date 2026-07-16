package competition

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/modules/scoreboard"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/validation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db         *gorm.DB
	scoreboard *scoreboard.Service
}

func New(db *gorm.DB, board *scoreboard.Service) *Service { return &Service{db: db, scoreboard: board} }

type View struct {
	ID                   uuid.UUID  `json:"id"`
	Slug                 string     `json:"slug"`
	Name                 string     `json:"name"`
	Summary              string     `json:"summary"`
	Description          string     `json:"description"`
	Mode                 string     `json:"mode"`
	Status               string     `json:"status"`
	ScoringMode          string     `json:"scoringMode"`
	Visibility           string     `json:"visibility"`
	BannerAssetKey       string     `json:"bannerAssetKey"`
	ThemeKey             string     `json:"themeKey"`
	RegistrationStartsAt time.Time  `json:"registrationStartsAt"`
	RegistrationEndsAt   time.Time  `json:"registrationEndsAt"`
	StartsAt             time.Time  `json:"startsAt"`
	EndsAt               time.Time  `json:"endsAt"`
	FreezeAt             *time.Time `json:"freezeAt,omitempty"`
	TeamMin              int        `json:"teamMin"`
	TeamMax              int        `json:"teamMax"`
	ParticipantCount     int64      `json:"participantCount"`
	ChallengeCount       int64      `json:"challengeCount"`
}
type Detail struct {
	View
	Challenges    []Challenge    `json:"challenges"`
	Announcements []Announcement `json:"announcements"`
	Appearance    map[string]any `json:"appearance"`
}
type Challenge struct {
	ID         uuid.UUID  `json:"id"`
	Slug       string     `json:"slug"`
	Title      string     `json:"title"`
	Category   string     `json:"category"`
	Difficulty string     `json:"difficulty"`
	Score      int        `json:"score"`
	SolveCount int64      `json:"solveCount"`
	Dynamic    bool       `json:"dynamic"`
	OpensAt    *time.Time `json:"opensAt,omitempty"`
}
type Announcement struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}
type Mutation struct {
	Slug, Name, Summary, Description, Mode, Status, ScoringMode, Visibility, BannerAssetKey, ThemeKey string
	RegistrationStartsAt, RegistrationEndsAt, StartsAt, EndsAt                                        time.Time
	FreezeAt                                                                                          *time.Time
	TeamMin, TeamMax                                                                                  int
	ChallengeIDs                                                                                      []uuid.UUID
}

func (s *Service) List(ctx context.Context, status string, page, size int, admin bool) (httpx.Page[View], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("competitions c")
	if !admin {
		query = query.Where("c.status<>'draft' AND c.status<>'archived'")
	} else if status != "" {
		query = query.Where("c.status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	var items []View
	selectSQL := `c.id,c.slug,c.name,c.summary,c.description_markdown AS description,c.mode,c.status,c.scoring_mode,c.visibility,c.banner_asset_key,c.theme_key,c.registration_starts_at,c.registration_ends_at,c.starts_at,c.ends_at,c.freeze_at,c.team_min,c.team_max,(SELECT count(*) FROM competition_participants cp WHERE cp.competition_id=c.id AND cp.status='registered')::bigint AS participant_count,(SELECT count(*) FROM competition_challenges cc WHERE cc.competition_id=c.id)::bigint AS challenge_count`
	if err := query.Select(selectSQL).Order("CASE c.status WHEN 'running' THEN 0 WHEN 'registration' THEN 1 WHEN 'draft' THEN 4 ELSE 2 END,c.starts_at DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	return httpx.Page[View]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Detail(ctx context.Context, identifier string, admin bool) (Detail, error) {
	var item View
	query := s.db.WithContext(ctx).Table("competitions c").Select(`c.id,c.slug,c.name,c.summary,c.description_markdown AS description,c.mode,c.status,c.scoring_mode,c.visibility,c.banner_asset_key,c.theme_key,c.registration_starts_at,c.registration_ends_at,c.starts_at,c.ends_at,c.freeze_at,c.team_min,c.team_max,(SELECT count(*) FROM competition_participants cp WHERE cp.competition_id=c.id AND cp.status='registered')::bigint AS participant_count,(SELECT count(*) FROM competition_challenges cc WHERE cc.competition_id=c.id)::bigint AS challenge_count`).Where("c.id::text=? OR c.slug=?", identifier, identifier)
	if !admin {
		query = query.Where("c.status<>'draft' AND c.status<>'archived'")
	}
	if err := query.First(&item).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
	}
	detail := Detail{View: item, Challenges: []Challenge{}, Announcements: []Announcement{}, Appearance: map[string]any{}}
	var snapshotState struct {
		CurrentSnapshotID *uuid.UUID
	}
	_ = s.db.WithContext(ctx).Table("competitions").Select("current_snapshot_id").Where("id=?", item.ID).Take(&snapshotState).Error
	currentSnapshotID := snapshotState.CurrentSnapshotID
	if !admin && currentSnapshotID != nil && (item.Status == "running" || item.Status == "frozen" || item.Status == "finished") {
		_ = s.db.WithContext(ctx).Table("competition_challenge_snapshots cs").Select("c.id,c.slug,r.title,d.name AS category,r.difficulty,cs.score,c.solve_count,r.is_dynamic AS dynamic,cs.opens_at").Joins("JOIN challenges c ON c.id=cs.challenge_id").Joins("JOIN challenge_revisions r ON r.id=cs.challenge_revision_id").Joins("LEFT JOIN challenge_directions d ON d.id=r.direction_id").Where("cs.competition_snapshot_id=?", *currentSnapshotID).Order("cs.sort_order").Scan(&detail.Challenges).Error
	} else {
		_ = s.db.WithContext(ctx).Table("competition_challenges cc").Select("c.id,c.slug,c.title,cat.name AS category,c.difficulty,cc.score,c.solve_count,c.is_dynamic AS dynamic,cc.opens_at").Joins("JOIN challenges c ON c.id=cc.challenge_id").Joins("JOIN challenge_categories cat ON cat.id=c.category_id").Where("cc.competition_id=?", item.ID).Order("cc.sort_order").Scan(&detail.Challenges).Error
	}
	_ = s.db.WithContext(ctx).Table("competition_announcements").Where("competition_id=? AND published_at IS NOT NULL", item.ID).Order("published_at DESC").Scan(&detail.Announcements).Error
	_ = s.db.WithContext(ctx).Table("competition_appearance").Where("competition_id=?", item.ID).Take(&detail.Appearance).Error
	return detail, nil
}
func (s *Service) Create(ctx context.Context, input Mutation) (Detail, error) {
	if input.Name == "" || !validation.In(input.Mode, "individual", "team") || input.ScoringMode != "" && !validation.In(input.ScoringMode, "fixed", "dynamic") {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_COMPETITION", "比赛名称或模式不合法")
	}
	if input.Slug == "" {
		input.Slug = validation.Slug(input.Name)
	}
	competition := models.Competition{ID: uuid.New(), Slug: input.Slug, Name: input.Name, Summary: input.Summary, DescriptionMarkdown: input.Description, Mode: input.Mode, Status: "draft", ScoringMode: input.ScoringMode, Visibility: input.Visibility, BannerAssetKey: input.BannerAssetKey, ThemeKey: input.ThemeKey, RegistrationStartsAt: input.RegistrationStartsAt, RegistrationEndsAt: input.RegistrationEndsAt, StartsAt: input.StartsAt, EndsAt: input.EndsAt, FreezeAt: input.FreezeAt, TeamMin: input.TeamMin, TeamMax: input.TeamMax}
	if competition.ScoringMode == "" {
		competition.ScoringMode = "dynamic"
	}
	if competition.Visibility == "" {
		competition.Visibility = "public"
	}
	if competition.TeamMin < 1 {
		competition.TeamMin = 1
	}
	if competition.TeamMax < competition.TeamMin {
		competition.TeamMax = max(5, competition.TeamMin)
	}
	if err := validateTimes(competition); err != nil {
		return Detail{}, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&competition).Error; err != nil {
			return err
		}
		return s.replaceChallenges(tx, competition.ID, input.ChallengeIDs)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Detail{}, httpx.NewError(http.StatusConflict, "COMPETITION_SLUG_EXISTS", "比赛 Slug 已存在，请换一个")
		}
		return Detail{}, err
	}
	return s.Detail(ctx, competition.ID.String(), true)
}
func (s *Service) Update(ctx context.Context, identifier string, input Mutation) (Detail, error) {
	var competition models.Competition
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&competition).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
	}
	if competition.Status != "draft" && competition.Status != "registration" {
		return Detail{}, httpx.NewError(http.StatusConflict, "COMPETITION_LOCKED", "比赛进入赛程后不能修改核心配置")
	}
	if input.Name == "" || !validation.In(input.Mode, "individual", "team") || !validation.In(input.ScoringMode, "fixed", "dynamic") {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_COMPETITION", "比赛名称或模式不合法")
	}
	candidate := competition
	candidate.Name, candidate.Summary, candidate.DescriptionMarkdown, candidate.Mode = input.Name, input.Summary, input.Description, input.Mode
	candidate.ScoringMode, candidate.Visibility = input.ScoringMode, input.Visibility
	candidate.RegistrationStartsAt, candidate.RegistrationEndsAt, candidate.StartsAt, candidate.EndsAt = input.RegistrationStartsAt, input.RegistrationEndsAt, input.StartsAt, input.EndsAt
	candidate.FreezeAt, candidate.TeamMin, candidate.TeamMax = input.FreezeAt, input.TeamMin, input.TeamMax
	if candidate.TeamMin < 1 || candidate.TeamMax < candidate.TeamMin {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_TEAM_SIZE", "战队人数范围不合法")
	}
	if err := validateTimes(candidate); err != nil {
		return Detail{}, err
	}
	updates := map[string]any{"name": input.Name, "summary": input.Summary, "description_markdown": input.Description, "mode": input.Mode, "scoring_mode": input.ScoringMode, "visibility": input.Visibility, "banner_asset_key": input.BannerAssetKey, "theme_key": input.ThemeKey, "registration_starts_at": input.RegistrationStartsAt, "registration_ends_at": input.RegistrationEndsAt, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "freeze_at": input.FreezeAt, "team_min": input.TeamMin, "team_max": input.TeamMax, "updated_at": time.Now().UTC()}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked models.Competition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id=?", competition.ID).Error; err != nil {
			return err
		}
		if locked.Status != "draft" && locked.Status != "registration" {
			return httpx.NewError(http.StatusConflict, "COMPETITION_LOCKED", "比赛进入赛程后不能修改核心配置")
		}
		if rosterConfigChanged(locked, input) {
			var participants int64
			if err := tx.Model(&models.CompetitionParticipant{}).Where("competition_id=? AND status='registered'", locked.ID).Count(&participants).Error; err != nil {
				return err
			}
			if participants > 0 {
				return httpx.NewError(http.StatusConflict, "REGISTERED_ROSTER_CONFIG_LOCKED", "已有报名后不能修改比赛模式或战队人数限制")
			}
		}
		if err := tx.Model(&locked).Updates(updates).Error; err != nil {
			return err
		}
		return s.replaceChallenges(tx, competition.ID, input.ChallengeIDs)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, competition.ID.String(), true)
}
func (s *Service) replaceChallenges(tx *gorm.DB, id uuid.UUID, challengeIDs []uuid.UUID) error {
	if err := tx.Where("competition_id=?", id).Delete(&models.CompetitionChallenge{}).Error; err != nil {
		return err
	}
	for index, challengeID := range challengeIDs {
		var challenge models.Challenge
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&challenge, "id=? AND status='published'", challengeID).Error; err != nil {
			return httpx.NewError(http.StatusBadRequest, "INVALID_CHALLENGE", "题目池包含未发布题目")
		}
		record := models.CompetitionChallenge{CompetitionID: id, ChallengeID: challengeID, Score: challenge.BaseScore, SortOrder: index}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Register(ctx context.Context, userID uuid.UUID, identifier string, teamID *uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var competition models.Competition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", identifier, identifier).First(&competition).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
		now := time.Now().UTC()
		if competition.Status != "registration" || now.Before(competition.RegistrationStartsAt) || now.After(competition.RegistrationEndsAt) {
			return httpx.NewError(http.StatusConflict, "REGISTRATION_CLOSED", "比赛当前不接受报名")
		}
		participant := models.CompetitionParticipant{ID: uuid.New(), CompetitionID: competition.ID, UserID: userID, Status: "registered", RegisteredAt: now, RosterSnapshot: []byte("[]")}
		var roster []MemberSnapshot
		if competition.Mode == "team" {
			if teamID == nil {
				return httpx.NewError(http.StatusBadRequest, "TEAM_REQUIRED", "团队赛必须选择战队")
			}
			var member models.TeamMember
			if err := tx.Where("team_id=? AND user_id=?", *teamID, userID).First(&member).Error; err != nil || !(member.Role == "captain" || member.Role == "manager") {
				return httpx.NewError(http.StatusForbidden, "TEAM_MANAGER_REQUIRED", "需要战队管理权限报名")
			}
			if err := tx.Table("team_members tm").Select("tm.user_id,u.username,tm.role").Joins("JOIN users u ON u.id=tm.user_id").Where("tm.team_id=?", *teamID).Order("tm.joined_at").Scan(&roster).Error; err != nil {
				return err
			}
			if len(roster) < competition.TeamMin || len(roster) > competition.TeamMax {
				return httpx.NewError(http.StatusConflict, "INVALID_ROSTER_SIZE", "战队人数不符合比赛要求")
			}
			participant.TeamID = teamID
			participant.RosterSnapshot, _ = json.Marshal(roster)
		} else if teamID != nil {
			return httpx.NewError(http.StatusBadRequest, "TEAM_NOT_ALLOWED", "个人赛不能使用战队报名")
		} else {
			var username string
			if err := tx.Table("users").Where("id=?", userID).Pluck("username", &username).Error; err != nil {
				return err
			}
			roster = []MemberSnapshot{{UserID: userID, Username: username, Role: "individual"}}
		}
		if err := tx.Create(&participant).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.NewError(http.StatusConflict, "ALREADY_REGISTERED", "已经报名该比赛")
			}
			return err
		}
		for _, member := range roster {
			record := map[string]any{"competition_id": competition.ID, "participant_id": participant.ID, "user_id": member.UserID, "username_snapshot": member.Username, "role_snapshot": member.Role, "registered_at": now}
			if err := tx.Table("competition_roster_members").Create(record).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return httpx.NewError(http.StatusConflict, "ROSTER_MEMBER_ALREADY_REGISTERED", "报名阵容中有成员已代表其他战队报名")
				}
				return err
			}
		}
		return nil
	})
}

type MemberSnapshot struct {
	UserID         uuid.UUID `json:"userId"`
	Username, Role string
}

func (s *Service) SetStatus(ctx context.Context, identifier, next string, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var competition models.Competition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", identifier, identifier).First(&competition).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
		allowed := map[string][]string{"draft": {"registration"}, "registration": {"running", "draft"}, "running": {"frozen", "finished"}, "frozen": {"finished"}, "finished": {"archived"}}
		if !contains(allowed[competition.Status], next) {
			return httpx.NewError(http.StatusConflict, "INVALID_COMPETITION_STATE", "比赛状态不能执行该迁移")
		}
		if next == "finished" {
			rows, err := s.scoreboard.AllCompetition(ctx, tx, competition.ID, nil)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(rows)
			snapshot := models.ScoreboardSnapshot{ID: uuid.New(), CompetitionID: competition.ID, Kind: "final", Payload: payload, Frozen: false, CreatedAt: time.Now().UTC()}
			if err := tx.Create(&snapshot).Error; err != nil {
				return err
			}
			settlement := map[string]any{"id": uuid.New(), "competition_id": competition.ID, "status": "completed", "snapshot_id": snapshot.ID, "settled_by": actor, "settled_at": time.Now().UTC(), "created_at": time.Now().UTC()}
			if err := tx.Table("competition_settlements").Create(settlement).Error; err != nil {
				return err
			}
		}
		if next == "frozen" {
			cutoff := time.Now().UTC()
			if competition.FreezeAt != nil && !competition.FreezeAt.After(cutoff) {
				cutoff = *competition.FreezeAt
			}
			rows, err := s.scoreboard.AllCompetition(ctx, tx, competition.ID, &cutoff)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(rows)
			if err := tx.Table("scoreboard_snapshots").Where("competition_id=? AND frozen=true", competition.ID).Update("frozen", false).Error; err != nil {
				return err
			}
			snapshot := models.ScoreboardSnapshot{ID: uuid.New(), CompetitionID: competition.ID, Kind: "freeze", Payload: payload, Frozen: true, CreatedAt: time.Now().UTC()}
			if err := tx.Create(&snapshot).Error; err != nil {
				return err
			}
		}
		if next == "running" {
			if err := s.createSnapshot(tx, &competition, actor); err != nil {
				return err
			}
		}
		return tx.Model(&competition).Updates(map[string]any{"status": next, "updated_at": time.Now().UTC()}).Error
	})
}

func (s *Service) createSnapshot(tx *gorm.DB, competition *models.Competition, actor uuid.UUID) error {
	var version int
	if err := tx.Table("competition_snapshots").Where("competition_id=?", competition.ID).Select("COALESCE(MAX(version),0)+1").Scan(&version).Error; err != nil {
		return err
	}
	competitionJSON, _ := json.Marshal(map[string]any{"name": competition.Name, "summary": competition.Summary, "description": competition.DescriptionMarkdown, "mode": competition.Mode, "scoringMode": competition.ScoringMode, "startsAt": competition.StartsAt, "endsAt": competition.EndsAt, "freezeAt": competition.FreezeAt})
	var rule struct{ Config json.RawMessage }
	if err := tx.Table("competition_scoring_rules").Select("config").Where("competition_id=?", competition.ID).Take(&rule).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if len(rule.Config) == 0 {
		rule.Config = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	createdBy := actor
	snapshot := models.CompetitionSnapshot{ID: uuid.New(), CompetitionID: competition.ID, Version: version, Status: "published", CompetitionJSON: competitionJSON, ScoringRulesJSON: rule.Config, CreatedBy: &createdBy, CreatedAt: now, EffectiveAt: now}
	if err := tx.Create(&snapshot).Error; err != nil {
		return err
	}
	var rows []models.CompetitionChallenge
	if err := tx.Where("competition_id=?", competition.ID).Order("sort_order").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return httpx.NewError(http.StatusUnprocessableEntity, "COMPETITION_HAS_NO_CHALLENGES", "比赛至少需要一道已发布题目")
	}
	for _, row := range rows {
		var challenge models.Challenge
		if err := tx.First(&challenge, "id=?", row.ChallengeID).Error; err != nil || challenge.CurrentPublishedRevisionID == nil {
			return httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_REVISION_REQUIRED", "比赛题目必须先发布不可变版本")
		}
		var runtimeState struct{ ID uuid.UUID }
		result := tx.Table("challenge_runtime_revisions").Select("id").Where("challenge_revision_id=?", *challenge.CurrentPublishedRevisionID).Take(&runtimeState)
		var runtimeRevisionID *uuid.UUID
		if result.Error == nil {
			runtimeRevisionID = &runtimeState.ID
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		record := models.CompetitionChallengeSnapshot{ID: uuid.New(), CompetitionSnapshotID: snapshot.ID, ChallengeID: row.ChallengeID, ChallengeRevisionID: *challenge.CurrentPublishedRevisionID, RuntimeRevisionID: runtimeRevisionID, Score: row.Score, SortOrder: row.SortOrder, OpensAt: row.OpensAt, CreatedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}
	competition.CurrentSnapshotID = &snapshot.ID
	return tx.Model(competition).Update("current_snapshot_id", snapshot.ID).Error
}
func validateTimes(c models.Competition) error {
	if c.RegistrationStartsAt.IsZero() || !c.RegistrationStartsAt.Before(c.RegistrationEndsAt) || !c.RegistrationEndsAt.Before(c.StartsAt) || !c.StartsAt.Before(c.EndsAt) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_COMPETITION_TIME", "报名和比赛时间顺序不正确")
	}
	if c.FreezeAt != nil && (!c.FreezeAt.After(c.StartsAt) || !c.FreezeAt.Before(c.EndsAt)) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_FREEZE_TIME", "封榜时间必须位于比赛期间")
	}
	return nil
}

func rosterConfigChanged(current models.Competition, input Mutation) bool {
	return current.Mode != input.Mode || current.TeamMin != input.TeamMin || current.TeamMax != input.TeamMax
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
