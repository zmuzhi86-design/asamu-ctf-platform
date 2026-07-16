package scoreboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CorrectionResult struct {
	EventID      uuid.UUID `json:"eventId"`
	CorrectionID uuid.UUID `json:"correctionId"`
	Delta        int       `json:"delta"`
}

type RebuildResult struct {
	TeamsUpdated      int64 `json:"teamsUpdated"`
	ChallengesUpdated int64 `json:"challengesUpdated"`
}
type AdjustmentInput struct {
	UserID        *uuid.UUID `json:"userId"`
	TeamID        *uuid.UUID `json:"teamId,omitempty"`
	CompetitionID *uuid.UUID `json:"competitionId,omitempty"`
	ChallengeID   *uuid.UUID `json:"challengeId,omitempty"`
	Delta         int        `json:"delta"`
	Reason        string     `json:"reason"`
}
type AdjustmentResult struct {
	EventID uuid.UUID `json:"eventId"`
	Delta   int       `json:"delta"`
}
type EventRow struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"userId"`
	Username       string     `json:"username"`
	TeamID         *uuid.UUID `json:"teamId,omitempty"`
	CompetitionID  *uuid.UUID `json:"competitionId,omitempty"`
	ChallengeID    *uuid.UUID `json:"challengeId,omitempty"`
	TeamName       string     `json:"teamName"`
	ChallengeTitle string     `json:"challengeTitle"`
	Type           string     `json:"type"`
	ReferenceType  string     `json:"referenceType"`
	Reason         string     `json:"reason"`
	Delta          int        `json:"delta"`
	Corrected      bool       `json:"corrected"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Row struct {
	Rank         int        `json:"rank"`
	SubjectType  string     `json:"subjectType"`
	SubjectID    uuid.UUID  `json:"subjectId"`
	UserID       uuid.UUID  `json:"userId"`
	Username     string     `json:"username"`
	TeamID       *uuid.UUID `json:"teamId,omitempty"`
	TeamName     string     `json:"teamName,omitempty"`
	Organization string     `json:"organization"`
	Score        int64      `json:"score"`
	Solves       int64      `json:"solves"`
	Bloods       int64      `json:"bloods"`
	LastSolveAt  *time.Time `json:"lastSolveAt,omitempty"`
}

func (s *Service) Global(ctx context.Context, page, pageSize int) (httpx.Page[Row], error) {
	return s.individualRows(ctx, nil, nil, page, pageSize)
}

type competitionState struct {
	Mode      string
	Status    string
	FreezeAt  *time.Time
	UpdatedAt time.Time
}

type publicView struct {
	SnapshotKind string
	Cutoff       *time.Time
}

func planPublicView(competition competitionState, now time.Time) publicView {
	switch competition.Status {
	case "finished", "archived":
		return publicView{SnapshotKind: "final", Cutoff: &competition.UpdatedAt}
	case "frozen":
		return publicView{SnapshotKind: "freeze", Cutoff: &competition.UpdatedAt}
	case "running":
		if competition.FreezeAt != nil && !now.Before(*competition.FreezeAt) {
			return publicView{Cutoff: competition.FreezeAt}
		}
	}
	return publicView{}
}

func (s *Service) Competition(ctx context.Context, competitionID uuid.UUID, page, pageSize int, public bool) (httpx.Page[Row], error) {
	var competition competitionState
	if err := s.db.WithContext(ctx).Table("competitions").Select("mode,status,freeze_at,updated_at").Where("id=?", competitionID).First(&competition).Error; err != nil {
		return httpx.Page[Row]{}, err
	}
	var cutoff *time.Time
	if public {
		view := planPublicView(competition, time.Now().UTC())
		cutoff = view.Cutoff
		if view.SnapshotKind != "" {
			rows, found, err := s.snapshotRows(ctx, competitionID, competition.Mode, view.SnapshotKind)
			if err != nil {
				return httpx.Page[Row]{}, err
			}
			if found {
				return pageRows(rows, page, pageSize), nil
			}
		}
	}
	return s.competitionRows(ctx, competitionID, competition.Mode, cutoff, page, pageSize)
}

// AllCompetition reads a complete live scoreboard through the supplied
// transaction. Competition settlement uses this so state transitions, score
// writes, and the final/freeze snapshot share one database view and no 10k row
// cap can truncate the snapshot.
func (s *Service) AllCompetition(ctx context.Context, tx *gorm.DB, competitionID uuid.UUID, cutoff *time.Time) ([]Row, error) {
	if tx == nil {
		tx = s.db
	}
	scoped := New(tx)
	var competition struct{ Mode string }
	if err := tx.WithContext(ctx).Table("competitions").Select("mode").Where("id=?", competitionID).First(&competition).Error; err != nil {
		return nil, err
	}
	first, err := scoped.competitionRows(ctx, competitionID, competition.Mode, cutoff, 1, 1)
	if err != nil {
		return nil, err
	}
	if first.Total == 0 {
		return []Row{}, nil
	}
	maxInt := int64(^uint(0) >> 1)
	if first.Total > maxInt {
		return nil, errors.New("competition scoreboard is too large to snapshot")
	}
	all, err := scoped.competitionRows(ctx, competitionID, competition.Mode, cutoff, 1, int(first.Total))
	if err != nil {
		return nil, err
	}
	return all.Items, nil
}

func (s *Service) snapshotRows(ctx context.Context, competitionID uuid.UUID, mode, kind string) ([]Row, bool, error) {
	var snapshot struct{ Payload []byte }
	query := s.db.WithContext(ctx).Table("scoreboard_snapshots ss").Select("ss.payload")
	if kind == "final" {
		query = query.Joins("JOIN competition_settlements settlement ON settlement.snapshot_id=ss.id").Where("settlement.competition_id=? AND settlement.status='completed' AND ss.kind='final'", competitionID).Order("settlement.settled_at DESC,ss.created_at DESC")
	} else {
		query = query.Where("ss.competition_id=? AND ss.kind='freeze' AND ss.frozen=true", competitionID).Order("ss.created_at DESC")
	}
	if err := query.Take(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var rows []Row
	if err := json.Unmarshal(snapshot.Payload, &rows); err != nil {
		return nil, false, nil
	}
	if err := normalizeSnapshotRows(rows, mode); err != nil {
		// Older team snapshots contained user rows and cannot safely represent a
		// team scoreboard. Replaying events at the state cutoff is deterministic.
		return nil, false, nil
	}
	return rows, true, nil
}

func normalizeSnapshotRows(rows []Row, mode string) error {
	for index := range rows {
		row := &rows[index]
		if row.Rank == 0 {
			row.Rank = index + 1
		}
		if mode == "team" {
			if row.SubjectType == "" && row.TeamID != nil {
				row.SubjectType = "team"
			}
			if row.SubjectType != "team" {
				return errors.New("team scoreboard snapshot has no explicit team subject")
			}
			if row.SubjectID == uuid.Nil && row.TeamID != nil {
				row.SubjectID = *row.TeamID
			}
			if row.SubjectID == uuid.Nil {
				return errors.New("team scoreboard snapshot has an empty subject id")
			}
			if row.TeamID != nil && *row.TeamID != row.SubjectID {
				return errors.New("team scoreboard snapshot has inconsistent subject ids")
			}
			teamID := row.SubjectID
			row.TeamID = &teamID
			row.UserID = row.SubjectID
			if row.TeamName == "" {
				row.TeamName = row.Username
			}
			if row.Username == "" {
				row.Username = row.TeamName
			}
			continue
		}
		if row.SubjectType == "" {
			row.SubjectType = "user"
		}
		if row.SubjectType != "user" {
			return errors.New("individual scoreboard snapshot has a non-user subject")
		}
		if row.SubjectID == uuid.Nil {
			row.SubjectID = row.UserID
		}
		if row.SubjectID == uuid.Nil {
			return errors.New("individual scoreboard snapshot has an empty subject id")
		}
		if row.UserID != uuid.Nil && row.UserID != row.SubjectID {
			return errors.New("individual scoreboard snapshot has inconsistent subject ids")
		}
		row.UserID = row.SubjectID
	}
	return nil
}

func (s *Service) competitionRows(ctx context.Context, competitionID uuid.UUID, mode string, cutoff *time.Time, page, pageSize int) (httpx.Page[Row], error) {
	if mode == "team" {
		return s.teamRows(ctx, competitionID, cutoff, page, pageSize)
	}
	return s.individualRows(ctx, &competitionID, cutoff, page, pageSize)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

func (s *Service) individualRows(ctx context.Context, competitionID *uuid.UUID, cutoff *time.Time, page, pageSize int) (httpx.Page[Row], error) {
	page, pageSize = normalizePage(page, pageSize)
	var competitionArg any
	if competitionID != nil {
		competitionArg = *competitionID
	}
	var total int64
	countQuery := `SELECT count(DISTINCT user_id)
FROM score_events
WHERE competition_id IS NOT DISTINCT FROM ?::uuid
  AND (?::timestamptz IS NULL OR created_at <= ?::timestamptz)`
	if err := s.db.WithContext(ctx).Raw(countQuery, competitionArg, cutoff, cutoff).Scan(&total).Error; err != nil {
		return httpx.Page[Row]{}, err
	}
	var rows []Row
	query := `WITH params AS (
  SELECT ?::uuid AS competition_id, ?::timestamptz AS cutoff
), totals AS (
	  SELECT user_id, sum(delta)::bigint AS score
	  FROM score_events se, params p
	  WHERE se.competition_id IS NOT DISTINCT FROM p.competition_id
	    AND (p.cutoff IS NULL OR se.created_at <= p.cutoff)
	  GROUP BY user_id
)
SELECT totals.user_id,u.username,coalesce(up.organization_name,'') AS organization,totals.score,
	  (SELECT count(*)::bigint FROM solve_records sr, params p WHERE sr.user_id=totals.user_id AND sr.competition_id IS NOT DISTINCT FROM p.competition_id AND (p.cutoff IS NULL OR sr.solved_at <= p.cutoff)) AS solves,
	  (SELECT count(*)::bigint FROM blood_records br, params p WHERE br.user_id=totals.user_id AND br.rank=1 AND br.competition_id IS NOT DISTINCT FROM p.competition_id AND (p.cutoff IS NULL OR br.created_at <= p.cutoff)) AS bloods,
	  (SELECT max(sr.solved_at) FROM solve_records sr, params p WHERE sr.user_id=totals.user_id AND sr.competition_id IS NOT DISTINCT FROM p.competition_id AND (p.cutoff IS NULL OR sr.solved_at <= p.cutoff)) AS last_solve_at
FROM totals
JOIN users u ON u.id=totals.user_id
LEFT JOIN user_profiles up ON up.user_id=u.id
ORDER BY totals.score DESC,last_solve_at ASC,u.username ASC
LIMIT ? OFFSET ?`
	if err := s.db.WithContext(ctx).Raw(query, competitionArg, cutoff, pageSize, (page-1)*pageSize).Scan(&rows).Error; err != nil {
		return httpx.Page[Row]{}, err
	}
	for index := range rows {
		rows[index].Rank = (page-1)*pageSize + index + 1
		rows[index].SubjectType = "user"
		rows[index].SubjectID = rows[index].UserID
	}
	return httpx.Page[Row]{Items: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (s *Service) teamRows(ctx context.Context, competitionID uuid.UUID, cutoff *time.Time, page, pageSize int) (httpx.Page[Row], error) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	if err := s.db.WithContext(ctx).Table("competition_participants").Where("competition_id=? AND team_id IS NOT NULL AND status='registered'", competitionID).Distinct("team_id").Count(&total).Error; err != nil {
		return httpx.Page[Row]{}, err
	}
	query := `WITH params AS (
  SELECT ?::uuid AS competition_id, ?::timestamptz AS cutoff
), registered AS (
  SELECT DISTINCT cp.team_id
  FROM competition_participants cp, params p
  WHERE cp.competition_id=p.competition_id AND cp.team_id IS NOT NULL AND cp.status='registered'
), totals AS (
  SELECT se.team_id,sum(se.delta)::bigint AS score
  FROM score_events se, params p
  WHERE se.competition_id=p.competition_id AND se.team_id IS NOT NULL
    AND (p.cutoff IS NULL OR se.created_at <= p.cutoff)
  GROUP BY se.team_id
), solves AS (
  SELECT sr.team_id,count(DISTINCT sr.challenge_id)::bigint AS solves,max(sr.solved_at) AS last_solve_at
  FROM solve_records sr, params p
  WHERE sr.competition_id=p.competition_id AND sr.team_id IS NOT NULL
    AND (p.cutoff IS NULL OR sr.solved_at <= p.cutoff)
  GROUP BY sr.team_id
), bloods AS (
  SELECT submission.team_id,count(*)::bigint AS bloods
  FROM blood_records br
  JOIN submissions submission ON submission.id=br.submission_id
  CROSS JOIN params p
  WHERE br.competition_id=p.competition_id AND br.rank=1 AND submission.team_id IS NOT NULL
    AND (p.cutoff IS NULL OR br.created_at <= p.cutoff)
  GROUP BY submission.team_id
)
SELECT registered.team_id AS subject_id,registered.team_id AS user_id,registered.team_id AS team_id,
  'team' AS subject_type,t.name AS username,t.name AS team_name,'' AS organization,
  coalesce(totals.score,0)::bigint AS score,coalesce(solves.solves,0)::bigint AS solves,
  coalesce(bloods.bloods,0)::bigint AS bloods,solves.last_solve_at
FROM registered
JOIN teams t ON t.id=registered.team_id
LEFT JOIN totals ON totals.team_id=registered.team_id
LEFT JOIN solves ON solves.team_id=registered.team_id
LEFT JOIN bloods ON bloods.team_id=registered.team_id
ORDER BY score DESC,last_solve_at ASC,t.name ASC
LIMIT ? OFFSET ?`
	var rows []Row
	if err := s.db.WithContext(ctx).Raw(query, competitionID, cutoff, pageSize, (page-1)*pageSize).Scan(&rows).Error; err != nil {
		return httpx.Page[Row]{}, err
	}
	for index := range rows {
		rows[index].Rank = (page-1)*pageSize + index + 1
		// Keep the legacy user fields populated so existing competition clients
		// continue rendering a team row without a coordinated frontend rollout.
		rows[index].SubjectType = "team"
		rows[index].SubjectID = rows[index].UserID
		teamID := rows[index].SubjectID
		rows[index].TeamID = &teamID
		if rows[index].TeamName == "" {
			rows[index].TeamName = rows[index].Username
		}
	}
	return httpx.Page[Row]{Items: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}
func pageRows(rows []Row, page, size int) httpx.Page[Row] {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	start := (page - 1) * size
	if start > len(rows) {
		start = len(rows)
	}
	end := start + size
	if end > len(rows) {
		end = len(rows)
	}
	return httpx.Page[Row]{Items: rows[start:end], Page: page, PageSize: size, Total: int64(len(rows)), TotalPages: (len(rows) + size - 1) / size}
}

func lockActiveCompetition(tx *gorm.DB, competitionID uuid.UUID) error {
	var competition struct {
		Status           string
		StartsAt, EndsAt time.Time
	}
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Table("competitions").Select("status,starts_at,ends_at").Where("id=?", competitionID).Take(&competition).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.NewError(http.StatusNotFound, "COMPETITION_NOT_FOUND", "比赛不存在")
		}
		return err
	}
	now := time.Now().UTC()
	if (competition.Status != "running" && competition.Status != "frozen") || now.Before(competition.StartsAt) || !now.Before(competition.EndsAt) {
		return httpx.NewError(http.StatusConflict, "COMPETITION_SCORING_LOCKED", "比赛当前不能调整积分")
	}
	return nil
}

func (s *Service) VoidEvent(ctx context.Context, eventID, actor uuid.UUID, reason string) (CorrectionResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 500 {
		return CorrectionResult{}, httpx.NewError(http.StatusBadRequest, "INVALID_REASON", "调整原因需要 4 到 500 个字符")
	}
	var result CorrectionResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "score-event:"+eventID.String()).Error; err != nil {
			return err
		}
		var original struct {
			ID, UserID                         uuid.UUID
			TeamID, CompetitionID, ChallengeID *uuid.UUID
			Type                               string
			Delta                              int
		}
		if err := tx.Table("score_events").Where("id=?", eventID).Take(&original).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return httpx.NewError(http.StatusNotFound, "SCORE_EVENT_NOT_FOUND", "积分事件不存在")
			}
			return err
		}
		if original.Type == "correction" {
			return httpx.NewError(http.StatusConflict, "CORRECTION_IMMUTABLE", "不能再次调整更正事件")
		}
		if original.CompetitionID != nil {
			if err := lockActiveCompetition(tx, *original.CompetitionID); err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Table("score_events").Where("parent_event_id=?", eventID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return httpx.NewError(http.StatusConflict, "EVENT_ALREADY_VOIDED", "积分事件已经调整")
		}
		correctionID := uuid.New()
		snapshot, _ := json.Marshal(map[string]any{"sourceEventId": eventID, "sourceType": original.Type, "sourceDelta": original.Delta})
		correction := map[string]any{"id": correctionID, "user_id": original.UserID, "team_id": original.TeamID, "competition_id": original.CompetitionID, "challenge_id": original.ChallengeID, "type": "correction", "delta": -original.Delta, "reference_type": "score_event", "reference_id": eventID, "rule_snapshot": snapshot, "parent_event_id": eventID, "reason": reason, "created_by": actor, "created_at": time.Now().UTC()}
		if err := tx.Table("score_events").Create(correction).Error; err != nil {
			return err
		}
		if original.TeamID != nil {
			if err := tx.Table("teams").Where("id=?", *original.TeamID).Update("score", gorm.Expr("score-?", original.Delta)).Error; err != nil {
				return err
			}
		}
		result = CorrectionResult{EventID: eventID, CorrectionID: correctionID, Delta: -original.Delta}
		return nil
	})
	return result, err
}

func (s *Service) RebuildDerived(ctx context.Context) (RebuildResult, error) {
	var result RebuildResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('scoreboard-rebuild'))").Error; err != nil {
			return err
		}
		teams := tx.Exec(`UPDATE teams t SET score=COALESCE((SELECT sum(se.delta) FROM score_events se WHERE se.team_id=t.id),0)`)
		if teams.Error != nil {
			return teams.Error
		}
		challenges := tx.Exec(`UPDATE challenges c SET solve_count=(SELECT count(*) FROM solve_records sr WHERE sr.challenge_id=c.id),attempt_count=(SELECT count(*) FROM submissions s WHERE s.challenge_id=c.id)`)
		if challenges.Error != nil {
			return challenges.Error
		}
		result = RebuildResult{TeamsUpdated: teams.RowsAffected, ChallengesUpdated: challenges.RowsAffected}
		return nil
	})
	return result, err
}

func (s *Service) Adjust(ctx context.Context, actor uuid.UUID, input AdjustmentInput) (AdjustmentResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.UserID == nil {
		return AdjustmentResult{}, httpx.NewError(http.StatusBadRequest, "USER_REQUIRED", "必须指定积分归属用户")
	}
	if input.Delta == 0 || input.Delta < -100000 || input.Delta > 100000 {
		return AdjustmentResult{}, httpx.NewError(http.StatusBadRequest, "INVALID_DELTA", "单次积分调整范围为 -100000 到 100000 且不能为零")
	}
	if len(input.Reason) < 4 || len(input.Reason) > 500 {
		return AdjustmentResult{}, httpx.NewError(http.StatusBadRequest, "INVALID_REASON", "调整原因需要 4 到 500 个字符")
	}
	eventID := uuid.New()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("users").Where("id=? AND status='active'", *input.UserID).Count(&count).Error; err != nil || count == 0 {
			return httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "积分归属用户不存在或不可用")
		}
		if input.TeamID != nil {
			if err := tx.Table("team_members").Where("team_id=? AND user_id=?", *input.TeamID, *input.UserID).Count(&count).Error; err != nil || count == 0 {
				return httpx.NewError(http.StatusBadRequest, "TEAM_MEMBERSHIP_REQUIRED", "积分归属用户不是该战队成员")
			}
		}
		if input.CompetitionID != nil {
			if err := lockActiveCompetition(tx, *input.CompetitionID); err != nil {
				return err
			}
		}
		if input.ChallengeID != nil {
			if err := tx.Table("challenges").Where("id=?", *input.ChallengeID).Count(&count).Error; err != nil || count == 0 {
				return httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
			}
		}
		snapshot, _ := json.Marshal(map[string]any{"kind": "manual_adjustment", "reason": input.Reason})
		event := models.ScoreEvent{ID: eventID, UserID: *input.UserID, TeamID: input.TeamID, CompetitionID: input.CompetitionID, ChallengeID: input.ChallengeID, Type: "admin_adjustment", Delta: input.Delta, ReferenceType: "score_adjustment", ReferenceID: eventID, RuleSnapshot: snapshot, Reason: input.Reason, CreatedBy: &actor, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if input.TeamID != nil {
			if err := tx.Table("teams").Where("id=?", *input.TeamID).Update("score", gorm.Expr("score+?", input.Delta)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return AdjustmentResult{EventID: eventID, Delta: input.Delta}, err
}

func (s *Service) Events(ctx context.Context, userID, competitionID *uuid.UUID, eventType string, page, size int) (httpx.Page[EventRow], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	query := s.db.WithContext(ctx).Table("score_events se").Joins("JOIN users u ON u.id=se.user_id").Joins("LEFT JOIN teams t ON t.id=se.team_id").Joins("LEFT JOIN challenges c ON c.id=se.challenge_id")
	if userID != nil {
		query = query.Where("se.user_id=?", *userID)
	}
	if competitionID != nil {
		query = query.Where("se.competition_id=?", *competitionID)
	}
	if eventType != "" {
		query = query.Where("se.type=?", eventType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[EventRow]{}, err
	}
	var rows []EventRow
	columns := `se.id,se.user_id,u.username,se.team_id,se.competition_id,se.challenge_id,coalesce(t.name,'') AS team_name,coalesce(c.title,'') AS challenge_title,se.type,se.delta,se.reference_type,se.reason,se.created_at,EXISTS(SELECT 1 FROM score_events correction WHERE correction.parent_event_id=se.id) AS corrected`
	if err := query.Select(columns).Order("se.created_at DESC").Offset((page - 1) * size).Limit(size).Scan(&rows).Error; err != nil {
		return httpx.Page[EventRow]{}, err
	}
	return httpx.Page[EventRow]{Items: rows, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
