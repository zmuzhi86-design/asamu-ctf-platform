package submission

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/modules/scoreboard"
	"asamu.local/platform/api/internal/platform/competitionscope"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db         *gorm.DB
	redis      *redis.Client
	flagSecret []byte
}

func New(db *gorm.DB, redisClient *redis.Client, flagSecret string) *Service {
	return &Service{db: db, redis: redisClient, flagSecret: []byte(flagSecret)}
}

type SubmitInput struct {
	Flag          string     `json:"flag"`
	CompetitionID *uuid.UUID `json:"competitionId,omitempty"`
	TeamID        *uuid.UUID `json:"teamId,omitempty"`
}
type Result struct {
	SubmissionID uuid.UUID `json:"submissionId"`
	Correct      bool      `json:"correct"`
	Duplicate    bool      `json:"duplicate"`
	AwardedScore int       `json:"awardedScore"`
	BloodRank    int       `json:"bloodRank,omitempty"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Service) Submit(ctx context.Context, userID uuid.UUID, challengeIdentifier string, input SubmitInput, ip, userAgent, requestID string) (Result, error) {
	candidate := strings.TrimSpace(input.Flag)
	if len(candidate) < 6 || len(candidate) > 512 {
		return Result{}, httpx.NewError(http.StatusBadRequest, "INVALID_FLAG_FORMAT", "Flag 格式不正确")
	}
	if input.CompetitionID == nil && input.TeamID != nil {
		return Result{}, httpx.NewError(http.StatusBadRequest, "TEAM_SCOPE_NOT_ALLOWED", "全局题目提交不能指定战队")
	}
	allowed, err := s.allow(ctx, userID, challengeIdentifier)
	if err != nil {
		return Result{}, err
	}
	if !allowed {
		return Result{}, httpx.NewError(http.StatusTooManyRequests, "SUBMISSION_RATE_LIMITED", "提交过于频繁，请稍后再试")
	}
	var challenge models.Challenge
	if err := s.db.WithContext(ctx).Where("(id::text=? OR slug=?) AND status='published'", challengeIdentifier, challengeIdentifier).First(&challenge).Error; err != nil {
		return Result{}, httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
	}
	if input.CompetitionID != nil {
		teamID, err := s.resolveCompetitionScope(ctx, userID, *input.CompetitionID, challenge.ID, input.TeamID)
		if err != nil {
			return Result{}, err
		}
		input.TeamID = teamID
	}
	revision, competitionSnapshotID, effectiveScoreMode, err := s.resolveRevision(ctx, challenge, input.CompetitionID)
	if err != nil {
		return Result{}, err
	}
	correct, instanceID, err := s.verify(ctx, userID, challenge, revision, input, candidate)
	if err != nil {
		return Result{}, err
	}
	fingerprint := s.fingerprint(candidate)
	challengeRevisionID := revision.ID
	submission := models.Submission{ID: uuid.New(), UserID: userID, ChallengeID: challenge.ID, TeamID: input.TeamID, CompetitionID: input.CompetitionID, InstanceID: instanceID, ChallengeRevisionID: &challengeRevisionID, CompetitionSnapshotID: competitionSnapshotID, Result: "incorrect", CandidateFingerprint: fingerprint, IP: ip, UserAgent: userAgent, RequestID: requestID}
	result := Result{SubmissionID: submission.ID, Correct: correct, Message: "Flag 不正确"}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.CompetitionID != nil {
			if _, err := competitionscope.ActiveChallenge(ctx, tx, *input.CompetitionID, challenge.ID, true); err != nil {
				return err
			}
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", solveLockKey(userID, input.TeamID, challenge.ID, input.CompetitionID)).Error; err != nil {
			return err
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "score:"+challenge.ID.String()+":"+optionalUUID(input.CompetitionID)).Error; err != nil {
			return err
		}
		// This timestamp is deliberately captured only after the competition
		// SHARE lock and scoring locks. Freeze/final snapshots can therefore
		// order every score consistently with the transaction that committed it.
		scoredAt := time.Now().UTC()
		submission.CreatedAt = scoredAt
		result.CreatedAt = scoredAt
		if err := tx.Model(&models.Challenge{}).Where("id=?", challenge.ID).Updates(map[string]any{"attempt_count": gorm.Expr("attempt_count+1"), "updated_at": scoredAt}).Error; err != nil {
			return err
		}
		if !correct {
			return tx.Create(&submission).Error
		}
		if input.TeamID != nil {
			claimed, err := claimTeamCompetitionSolve(tx, *input.CompetitionID, *input.TeamID, challenge.ID, scoredAt)
			if err != nil {
				return err
			}
			if !claimed {
				submission.Result = "duplicate"
				result.Duplicate = true
				result.Message = "战队已解出本题，本次提交不重复计分"
				return tx.Create(&submission).Error
			}
		} else {
			var existing models.SolveRecord
			findErr := tx.Where("user_id=? AND challenge_id=? AND competition_id IS NOT DISTINCT FROM ?", userID, challenge.ID, input.CompetitionID).First(&existing).Error
			if findErr == nil {
				submission.Result = "duplicate"
				result.Duplicate = true
				result.Message = "本题已经解出，本次提交不重复计分"
				return tx.Create(&submission).Error
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
		}
		scoreMaximum := revision.MaximumScore
		if competitionSnapshotID != nil {
			if err := tx.Table("competition_challenge_snapshots").Where("competition_snapshot_id=? AND challenge_id=?", *competitionSnapshotID, challenge.ID).Pluck("score", &scoreMaximum).Error; err != nil {
				return err
			}
		}
		score := revision.BaseScore
		if competitionSnapshotID != nil {
			score = scoreMaximum
		}
		if effectiveScoreMode == "dynamic" {
			var solveCount int64
			if input.TeamID != nil {
				if err := tx.Table("team_competition_solve_claims").Where("competition_id=? AND challenge_id=? AND solve_record_id IS NOT NULL", *input.CompetitionID, challenge.ID).Count(&solveCount).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&models.SolveRecord{}).Where("challenge_id=? AND competition_id IS NOT DISTINCT FROM ?", challenge.ID, input.CompetitionID).Count(&solveCount).Error; err != nil {
					return err
				}
			}
			minimum := revision.MinimumScore
			if scoreMaximum < minimum {
				minimum = scoreMaximum
			}
			score = scoreboard.DynamicScore(scoreboard.DynamicRule{Maximum: scoreMaximum, Minimum: minimum, Decay: revision.DynamicDecay}, solveCount)
		}
		submission.Result = "correct"
		submission.AwardedScore = score
		if err := tx.Create(&submission).Error; err != nil {
			return err
		}
		solve := models.SolveRecord{ID: uuid.New(), UserID: userID, ChallengeID: challenge.ID, TeamID: input.TeamID, CompetitionID: input.CompetitionID, SubmissionID: submission.ID, Score: score, SolvedAt: scoredAt}
		if err := tx.Create(&solve).Error; err != nil {
			return err
		}
		if input.TeamID != nil {
			attached := tx.Table("team_competition_solve_claims").Where("competition_id=? AND team_id=? AND challenge_id=? AND solve_record_id IS NULL", *input.CompetitionID, *input.TeamID, challenge.ID).Update("solve_record_id", solve.ID)
			if attached.Error != nil {
				return attached.Error
			}
			if attached.RowsAffected != 1 {
				return errors.New("team solve claim was not attached to its solve record")
			}
		}
		var bloodCount int64
		if err := tx.Model(&models.BloodRecord{}).Where("challenge_id=? AND competition_id IS NOT DISTINCT FROM ?", challenge.ID, input.CompetitionID).Count(&bloodCount).Error; err != nil {
			return err
		}
		bloodRank := 0
		bonus := 0
		if bloodCount < 3 {
			bloodRank = int(bloodCount) + 1
			bonus = scoreboard.BloodBonus(bloodRank, score)
			blood := models.BloodRecord{ID: uuid.New(), ChallengeID: challenge.ID, CompetitionID: input.CompetitionID, UserID: userID, SubmissionID: submission.ID, Rank: bloodRank, Award: bonus, CreatedAt: scoredAt}
			if err := tx.Create(&blood).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
				return err
			}
		}
		ruleSnapshot, _ := json.Marshal(map[string]any{"challengeRevisionId": revision.ID, "competitionSnapshotId": competitionSnapshotID, "scoreMode": effectiveScoreMode, "maximum": scoreMaximum, "minimum": revision.MinimumScore, "decay": revision.DynamicDecay, "baseAward": score, "bloodRank": bloodRank, "bloodBonus": bonus})
		event := models.ScoreEvent{ID: uuid.New(), UserID: userID, TeamID: input.TeamID, CompetitionID: input.CompetitionID, ChallengeID: &challenge.ID, Type: "solve", Delta: score + bonus, ReferenceType: "submission", ReferenceID: submission.ID, RuleSnapshot: ruleSnapshot, CreatedAt: scoredAt}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Challenge{}).Where("id=?", challenge.ID).Updates(map[string]any{"solve_count": gorm.Expr("solve_count+1"), "updated_at": scoredAt}).Error; err != nil {
			return err
		}
		if input.TeamID != nil {
			if err := tx.Model(&models.Team{}).Where("id=?", *input.TeamID).Update("score", gorm.Expr("score+?", score+bonus)).Error; err != nil {
				return err
			}
		}
		_ = tx.Table("experience_events").Create(map[string]any{"id": uuid.New(), "user_id": userID, "scheme_id": gorm.Expr("(SELECT id FROM level_schemes WHERE key='platform-default' LIMIT 1)"), "type": "challenge_solve", "delta": max(10, score/10), "reference_type": "submission", "reference_id": submission.ID, "created_at": scoredAt}).Error
		_ = tx.Create(&models.Notification{ID: uuid.New(), UserID: userID, Type: "challenge_solved", Title: "挑战完成", Body: "你已解出「" + challenge.Title + "」", Link: "/challenges/" + challenge.Slug, Payload: []byte(`{}`), CreatedAt: scoredAt}).Error
		result.AwardedScore = score + bonus
		result.BloodRank = bloodRank
		result.Message = "Flag 正确，积分已入账"
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	if shouldDetectSharedFlag(correct, revision.IsDynamic) {
		_ = s.detectSharedFlag(ctx, userID, fingerprint, submission.ID, submission.InstanceID)
	}
	return result, nil
}

func shouldDetectSharedFlag(correct, dynamic bool) bool {
	return correct && dynamic
}
func (s *Service) verify(ctx context.Context, userID uuid.UUID, challenge models.Challenge, revision models.ChallengeRevision, input SubmitInput, candidate string) (bool, *uuid.UUID, error) {
	if revision.IsDynamic {
		var instance models.ChallengeInstance
		ownerScope := "user"
		ownerID := userID
		if input.TeamID != nil {
			ownerScope = "team"
			ownerID = *input.TeamID
		}
		query := s.db.WithContext(ctx).Table("challenge_instances i").Select("i.*").Joins("JOIN challenge_runtime_revisions rr ON rr.id=i.runtime_revision_id").Where("i.challenge_id=? AND i.owner_scope=? AND i.owner_id=? AND i.status='running' AND rr.challenge_revision_id=?", challenge.ID, ownerScope, ownerID, revision.ID)
		if input.CompetitionID == nil {
			query = query.Where("competition_id IS NULL")
		} else {
			query = query.Where("competition_id=?", *input.CompetitionID)
		}
		if err := query.Order("created_at DESC").First(&instance).Error; err != nil {
			return false, nil, httpx.NewError(http.StatusConflict, "RUNNING_INSTANCE_REQUIRED", "请先启动属于你的动态环境")
		}
		return security.VerifyFlag(s.flagSecret, instance.FlagHMAC, candidate), &instance.ID, nil
	}
	var flags []models.ChallengeFlag
	if err := s.db.WithContext(ctx).Table("challenge_flag_revisions").Select("id,kind,hmac,regex_pattern,stage").Where("challenge_revision_id=?", revision.ID).Order("stage").Find(&flags).Error; err != nil {
		return false, nil, err
	}
	for _, flag := range flags {
		switch flag.Kind {
		case "static", "multi_stage":
			if security.VerifyFlag(s.flagSecret, flag.HMAC, candidate) {
				return true, nil, nil
			}
		case "regex":
			expression, err := regexp.Compile(flag.RegexPattern)
			if err != nil {
				return false, nil, httpx.NewError(http.StatusInternalServerError, "CHECKER_CONFIG_INVALID", "题目判定配置无效")
			}
			if expression.MatchString(candidate) {
				return true, nil, nil
			}
		case "checker":
			return false, nil, httpx.NewError(http.StatusServiceUnavailable, "CHECKER_ASYNC_REQUIRED", "该题使用隔离 Checker，请通过比赛判题队列提交")
		}
	}
	return false, nil, nil
}

func (s *Service) resolveRevision(ctx context.Context, challenge models.Challenge, competitionID *uuid.UUID) (models.ChallengeRevision, *uuid.UUID, string, error) {
	var revision models.ChallengeRevision
	if competitionID == nil {
		if challenge.CurrentPublishedRevisionID == nil {
			return revision, nil, "", httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_REVISION_REQUIRED", "题目尚未发布不可变版本")
		}
		if err := s.db.WithContext(ctx).First(&revision, "id=?", *challenge.CurrentPublishedRevisionID).Error; err != nil {
			return revision, nil, "", err
		}
		return revision, nil, revision.ScoreMode, nil
	}
	var row struct {
		CompetitionSnapshotID uuid.UUID
		ChallengeRevisionID   uuid.UUID
		CompetitionJSON       json.RawMessage
	}
	if err := s.db.WithContext(ctx).Table("competition_challenge_snapshots cs").Select("cs.competition_snapshot_id,cs.challenge_revision_id,s.competition_json").Joins("JOIN competition_snapshots s ON s.id=cs.competition_snapshot_id").Joins("JOIN competitions c ON c.current_snapshot_id=cs.competition_snapshot_id").Where("c.id=? AND cs.challenge_id=?", *competitionID, challenge.ID).Take(&row).Error; err != nil {
		return revision, nil, "", httpx.NewError(http.StatusConflict, "COMPETITION_SNAPSHOT_REQUIRED", "比赛题目快照不存在")
	}
	if err := s.db.WithContext(ctx).First(&revision, "id=?", row.ChallengeRevisionID).Error; err != nil {
		return revision, nil, "", err
	}
	return revision, &row.CompetitionSnapshotID, snapshotScoreMode(row.CompetitionJSON, revision.ScoreMode), nil
}

func snapshotScoreMode(raw json.RawMessage, fallback string) string {
	var snapshot struct {
		ScoringMode string `json:"scoringMode"`
	}
	if json.Unmarshal(raw, &snapshot) == nil && (snapshot.ScoringMode == "fixed" || snapshot.ScoringMode == "dynamic") {
		return snapshot.ScoringMode
	}
	return fallback
}
func (s *Service) resolveCompetitionScope(ctx context.Context, userID, competitionID, challengeID uuid.UUID, requestedTeamID *uuid.UUID) (*uuid.UUID, error) {
	competition, err := competitionscope.ActiveChallenge(ctx, s.db, competitionID, challengeID, false)
	if err != nil {
		return nil, err
	}
	var teamID *uuid.UUID
	if competition.Mode == "team" {
		participant, err := competitionscope.RegisteredTeam(ctx, s.db, competitionID, userID, requestedTeamID)
		if err != nil {
			if errors.Is(err, competitionscope.ErrAmbiguousTeam) {
				return nil, httpx.NewError(http.StatusConflict, "TEAM_SCOPE_AMBIGUOUS", "当前比赛存在多个报名战队，请明确指定战队")
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "不在该比赛的报名阵容中")
			}
			return nil, err
		}
		teamID = participant.TeamID
	} else {
		if requestedTeamID != nil {
			return nil, httpx.NewError(http.StatusBadRequest, "TEAM_SCOPE_NOT_ALLOWED", "个人赛不能指定战队")
		}
		var participant models.CompetitionParticipant
		if err := s.db.WithContext(ctx).Where("competition_id=? AND user_id=? AND team_id IS NULL AND status='registered'", competitionID, userID).First(&participant).Error; err != nil {
			return nil, httpx.NewError(http.StatusForbidden, "NOT_COMPETITION_PARTICIPANT", "尚未报名该比赛")
		}
	}
	return teamID, nil
}

func (s *Service) allow(ctx context.Context, userID uuid.UUID, challenge string) (bool, error) {
	key := "submission:" + userID.String() + ":" + challenge
	script := redis.NewScript(`local n=redis.call('INCR',KEYS[1]);if n==1 then redis.call('EXPIRE',KEYS[1],60) end;if n>20 then return 0 else return 1 end`)
	value, err := script.Run(ctx, s.redis, []string{key}).Int()
	return value == 1, err
}
func (s *Service) fingerprint(candidate string) string {
	mac := hmac.New(sha256.New, s.flagSecret)
	_, _ = mac.Write([]byte(candidate))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) detectSharedFlag(ctx context.Context, userID uuid.UUID, fingerprint string, submissionID uuid.UUID, instanceID *uuid.UUID) error {
	var other models.Submission
	query := sharedFlagCandidateQuery(s.db.WithContext(ctx), userID, fingerprint, instanceID)
	err := query.Order("created_at").First(&other).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Table("cheat_events").Create(map[string]any{"id": uuid.New(), "user_id": userID, "risk_score": 90, "evidence": map[string]any{"type": "shared_dynamic_flag", "submissionId": submissionID, "previousSubmissionId": other.ID}, "created_at": time.Now().UTC()}).Error
}

func sharedFlagCandidateQuery(db *gorm.DB, userID uuid.UUID, fingerprint string, instanceID *uuid.UUID) *gorm.DB {
	query := db.Model(&models.Submission{}).Where("candidate_fingerprint=? AND user_id<>? AND result='correct'", fingerprint, userID)
	if instanceID != nil {
		query = query.Where("instance_id IS DISTINCT FROM ?", *instanceID)
	}
	return query
}
func optionalUUID(value *uuid.UUID) string {
	if value == nil {
		return "global"
	}
	return value.String()
}

func solveLockKey(userID uuid.UUID, teamID *uuid.UUID, challengeID uuid.UUID, competitionID *uuid.UUID) string {
	owner := "user:" + userID.String()
	if teamID != nil {
		owner = "team:" + teamID.String()
	}
	return owner + ":" + challengeID.String() + ":" + optionalUUID(competitionID)
}

func claimTeamCompetitionSolve(tx *gorm.DB, competitionID, teamID, challengeID uuid.UUID, claimedAt time.Time) (bool, error) {
	claim := tx.Table("team_competition_solve_claims").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"competition_id": competitionID,
		"team_id":        teamID,
		"challenge_id":   challengeID,
		"claimed_at":     claimedAt,
	})
	return claim.RowsAffected == 1, claim.Error
}
func (s *Service) History(ctx context.Context, userID uuid.UUID, challengeIdentifier string, page, pageSize int) (httpx.Page[models.Submission], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	query := s.db.WithContext(ctx).Model(&models.Submission{}).Where("user_id=? AND challenge_id IN (SELECT id FROM challenges WHERE id::text=? OR slug=?)", userID, challengeIdentifier, challengeIdentifier)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[models.Submission]{}, err
	}
	var items []models.Submission
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return httpx.Page[models.Submission]{}, err
	}
	return httpx.Page[models.Submission]{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}
