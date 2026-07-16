package user

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Profile struct {
	ID                 uuid.UUID        `json:"id"`
	Email              string           `json:"email,omitempty"`
	Username           string           `json:"username"`
	DisplayName        string           `json:"displayName"`
	Bio                string           `json:"bio"`
	OrganizationName   string           `json:"organizationName"`
	AvatarAssetKey     string           `json:"avatarAssetKey"`
	CharacterAssetKey  string           `json:"characterAssetKey"`
	Signature          string           `json:"signature"`
	Status             string           `json:"status"`
	Skills             []string         `json:"skills"`
	CreatedAt          time.Time        `json:"createdAt"`
	RecentSolves       []map[string]any `json:"recentSolves"`
	CompetitionHistory []map[string]any `json:"competitionHistory"`
	Favorites          []map[string]any `json:"favorites"`
	Privacy            map[string]any   `json:"privacy,omitempty"`
}
type Update struct {
	DisplayName, Bio, OrganizationName, AvatarAssetKey, CharacterAssetKey, Signature string
	Skills                                                                           []string
	Privacy                                                                          map[string]any
}

func (s *Service) Profile(ctx context.Context, identifier string, private bool) (Profile, error) {
	var row struct {
		ID                                                                                                        uuid.UUID
		Email, Username, DisplayName, Bio, OrganizationName, AvatarAssetKey, CharacterAssetKey, Signature, Status string
		Skills, Privacy                                                                                           []byte
		CreatedAt                                                                                                 time.Time
	}
	query := s.db.WithContext(ctx).Table("users u").Select("u.id,u.email,u.username,u.status,u.created_at,up.display_name,up.bio,up.organization_name,up.avatar_asset_key,up.character_asset_key,up.signature,up.skills,up.privacy").Joins("LEFT JOIN user_profiles up ON up.user_id=u.id").Where("(u.id::text=? OR u.username=?) AND u.deleted_at IS NULL", identifier, identifier)
	if err := query.First(&row).Error; err != nil {
		return Profile{}, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	}
	profile := Profile{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, Bio: row.Bio, OrganizationName: row.OrganizationName, AvatarAssetKey: row.AvatarAssetKey, CharacterAssetKey: row.CharacterAssetKey, Signature: row.Signature, Status: row.Status, CreatedAt: row.CreatedAt, Skills: []string{}, RecentSolves: []map[string]any{}, CompetitionHistory: []map[string]any{}, Favorites: []map[string]any{}}
	privacySettings := map[string]any{}
	_ = json.Unmarshal(row.Privacy, &privacySettings)
	if private {
		profile.Email = row.Email
		profile.Privacy = privacySettings
	}
	_ = json.Unmarshal(row.Skills, &profile.Skills)
	_ = s.db.WithContext(ctx).Table("solve_records sr").Select("c.slug,c.title,cat.name AS category,sr.score,sr.solved_at").Joins("JOIN challenges c ON c.id=sr.challenge_id").Joins("JOIN challenge_categories cat ON cat.id=c.category_id").Where("sr.user_id=?", row.ID).Order("sr.solved_at DESC").Limit(20).Find(&profile.RecentSolves).Error
	_ = s.db.WithContext(ctx).Table("competition_participants cp").Select("c.slug,c.name,c.status,cp.registered_at").Joins("JOIN competition_roster_members crm ON crm.participant_id=cp.id AND crm.competition_id=cp.competition_id").Joins("JOIN competitions c ON c.id=cp.competition_id").Where("crm.user_id=? AND cp.status='registered'", row.ID).Order("c.starts_at DESC").Limit(20).Find(&profile.CompetitionHistory).Error
	if private {
		_ = s.db.WithContext(ctx).Table("writeup_favorites wf").Select("w.slug,w.title,w.summary,wf.created_at").Joins("JOIN writeups w ON w.id=wf.writeup_id").Where("wf.user_id=?", row.ID).Order("wf.created_at DESC").Limit(20).Find(&profile.Favorites).Error
	} else {
		if privacyOff(privacySettings, "showOrganization") {
			profile.OrganizationName = ""
		}
		if privacyOff(privacySettings, "showSkills") {
			profile.Skills = []string{}
		}
		if privacyOff(privacySettings, "showRecentSolves") {
			profile.RecentSolves = []map[string]any{}
		}
		if privacyOff(privacySettings, "showCompetitionHistory") {
			profile.CompetitionHistory = []map[string]any{}
		}
	}
	return profile, nil
}
func privacyOff(settings map[string]any, key string) bool {
	value, exists := settings[key]
	if !exists {
		return false
	}
	enabled, ok := value.(bool)
	return ok && !enabled
}
func (s *Service) Update(ctx context.Context, userID uuid.UUID, input Update) (Profile, error) {
	if len(input.DisplayName) > 80 || len(input.Bio) > 2000 || len(input.OrganizationName) > 160 || len(input.Signature) > 500 || len(input.AvatarAssetKey) > 160 || len(input.CharacterAssetKey) > 160 || len(input.Skills) > 20 {
		return Profile{}, httpx.NewError(http.StatusBadRequest, "INVALID_PROFILE", "个人资料字段超过允许长度")
	}
	seen := map[string]bool{}
	cleanSkills := make([]string, 0, len(input.Skills))
	for _, skill := range input.Skills {
		skill = strings.TrimSpace(skill)
		if skill == "" || len(skill) > 50 || seen[strings.ToLower(skill)] {
			continue
		}
		seen[strings.ToLower(skill)] = true
		cleanSkills = append(cleanSkills, skill)
	}
	input.Skills = cleanSkills
	skills, _ := json.Marshal(input.Skills)
	privacy, _ := json.Marshal(input.Privacy)
	updates := map[string]any{"display_name": input.DisplayName, "bio": input.Bio, "organization_name": input.OrganizationName, "avatar_asset_key": input.AvatarAssetKey, "character_asset_key": input.CharacterAssetKey, "signature": input.Signature, "skills": skills, "privacy": privacy, "updated_at": time.Now().UTC()}
	if err := s.db.WithContext(ctx).Table("user_profiles").Where("user_id=?", userID).Updates(updates).Error; err != nil {
		return Profile{}, err
	}
	return s.Profile(ctx, userID.String(), true)
}
