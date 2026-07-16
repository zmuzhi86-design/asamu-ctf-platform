package team

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	assetmodule "asamu.local/platform/api/internal/modules/asset"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/security"
	"asamu.local/platform/api/internal/platform/validation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db              *gorm.DB
	assets          *assetmodule.Service
	confirmationKey []byte
	webBaseURL      string
}

func New(db *gorm.DB, confirmationSecret, webBaseURL string, assets *assetmodule.Service) *Service {
	key := sha256.Sum256([]byte(confirmationSecret))
	return &Service{db: db, assets: assets, confirmationKey: key[:], webBaseURL: strings.TrimRight(webBaseURL, "/")}
}

type View struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Slogan         string    `json:"slogan"`
	Description    string    `json:"description"`
	FlagAssetKey   string    `json:"flagAssetKey"`
	BannerAssetKey string    `json:"bannerAssetKey"`
	Recruiting     bool      `json:"recruiting"`
	CaptainID      uuid.UUID `json:"captainId"`
	CaptainName    string    `json:"captainName"`
	MemberCount    int64     `json:"memberCount"`
	MemberLimit    int       `json:"memberLimit"`
	Score          int64     `json:"score"`
	Rank           int64     `json:"rank"`
}
type Detail struct {
	View
	Members       []Member       `json:"members"`
	Announcements []Announcement `json:"announcements"`
	Honors        []Honor        `json:"honors"`
}
type Member struct {
	UserID   uuid.UUID `json:"userId"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}
type Announcement struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
}
type Honor struct {
	Name      string    `json:"name"`
	Rarity    string    `json:"rarity"`
	AssetKey  string    `json:"assetKey"`
	AwardedAt time.Time `json:"awardedAt"`
}
type CreateInput struct {
	Name, Slogan, Description, FlagAssetKey, BannerAssetKey string
	MemberLimit                                             int
}
type UpdateInput struct {
	Name, Slogan, Description, FlagAssetKey, BannerAssetKey string
	MemberLimit                                             int
	Recruiting                                              bool
}
type JoinRequestView struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
type InvitationView struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}
type Management struct {
	Team         Detail            `json:"team"`
	MyRole       string            `json:"myRole"`
	JoinRequests []JoinRequestView `json:"joinRequests"`
	Invitations  []InvitationView  `json:"invitations"`
}

func (s *Service) List(ctx context.Context, search string, recruiting *bool, page, size int) (httpx.Page[View], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("teams t").Joins("JOIN users u ON u.id=t.captain_id").Joins("LEFT JOIN team_members tm ON tm.team_id=t.id").Group("t.id,u.username")
	if search != "" {
		query = query.Where("t.name ILIKE ? OR t.slogan ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if recruiting != nil {
		query = query.Where("t.recruiting=?", *recruiting)
	}
	var total int64
	if err := s.db.WithContext(ctx).Table("(?) teams", query.Select("t.id")).Count(&total).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	var items []View
	selectSQL := `t.id,t.slug,t.name,t.slogan,t.description,t.flag_asset_key,t.banner_asset_key,t.recruiting,t.captain_id,u.username AS captain_name,count(tm.user_id)::bigint AS member_count,t.member_limit,t.score,(SELECT count(*)+1 FROM teams higher WHERE higher.score>t.score)::bigint AS rank`
	if err := query.Select(selectSQL).Order("t.score DESC,t.created_at").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	return httpx.Page[View]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Detail(ctx context.Context, identifier string) (Detail, error) {
	var item View
	query := s.db.WithContext(ctx).Table("teams t").Select(`t.id,t.slug,t.name,t.slogan,t.description,t.flag_asset_key,t.banner_asset_key,t.recruiting,t.captain_id,u.username AS captain_name,(SELECT count(*) FROM team_members WHERE team_id=t.id)::bigint AS member_count,t.member_limit,t.score,(SELECT count(*)+1 FROM teams higher WHERE higher.score>t.score)::bigint AS rank`).Joins("JOIN users u ON u.id=t.captain_id").Where("t.id::text=? OR t.slug=?", identifier, identifier)
	if err := query.First(&item).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusNotFound, "TEAM_NOT_FOUND", "战队不存在")
	}
	detail := Detail{View: item, Members: []Member{}, Announcements: []Announcement{}, Honors: []Honor{}}
	if err := s.db.WithContext(ctx).Table("team_members tm").Select("tm.user_id,u.username,tm.role,tm.joined_at").Joins("JOIN users u ON u.id=tm.user_id").Where("tm.team_id=?", item.ID).Order("CASE tm.role WHEN 'captain' THEN 0 WHEN 'manager' THEN 1 ELSE 2 END,tm.joined_at").Scan(&detail.Members).Error; err != nil {
		return Detail{}, err
	}
	if err := s.db.WithContext(ctx).Table("team_announcements").Select("id,title,content,pinned,created_at").Where("team_id=?", item.ID).Order("pinned DESC,created_at DESC").Limit(20).Scan(&detail.Announcements).Error; err != nil {
		return Detail{}, err
	}
	if err := s.db.WithContext(ctx).Table("team_medals um").Select("m.name,m.rarity,m.unlocked_asset_key AS asset_key,um.awarded_at").Joins("JOIN medals m ON m.id=um.medal_id").Where("um.team_id=? AND um.revoked_at IS NULL", item.ID).Order("um.awarded_at DESC").Scan(&detail.Honors).Error; err != nil {
		return Detail{}, err
	}
	return detail, nil
}
func (s *Service) Manage(ctx context.Context, userID uuid.UUID) (Management, error) {
	var member models.TeamMember
	if err := s.db.WithContext(ctx).Where("user_id=?", userID).First(&member).Error; err != nil {
		return Management{}, httpx.NewError(http.StatusNotFound, "TEAM_MEMBERSHIP_NOT_FOUND", "你当前未加入战队")
	}
	detail, err := s.Detail(ctx, member.TeamID.String())
	if err != nil {
		return Management{}, err
	}
	result := Management{Team: detail, MyRole: member.Role, JoinRequests: []JoinRequestView{}, Invitations: []InvitationView{}}
	if member.Role == "captain" || member.Role == "manager" {
		if err := s.db.WithContext(ctx).Table("team_join_requests r").Select("r.id,r.user_id,u.username,r.message,r.status,r.created_at").Joins("JOIN users u ON u.id=r.user_id").Where("r.team_id=? AND r.status='pending'", member.TeamID).Order("r.created_at").Scan(&result.JoinRequests).Error; err != nil {
			return Management{}, err
		}
		if err := s.db.WithContext(ctx).Table("team_invitations i").Select("i.id,i.user_id,u.username,i.status,i.expires_at,i.created_at").Joins("JOIN users u ON u.id=i.user_id").Where("i.team_id=? AND i.status='pending' AND i.expires_at>?", member.TeamID, time.Now().UTC()).Order("i.created_at DESC").Scan(&result.Invitations).Error; err != nil {
			return Management{}, err
		}
	}
	return result, nil
}
func (s *Service) Update(ctx context.Context, actorID uuid.UUID, identifier string, input UpdateInput) (Detail, error) {
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 80 || len(input.Slogan) > 240 || len(input.Description) > 5000 || len(input.FlagAssetKey) > 160 || len(input.BannerAssetKey) > 160 || input.MemberLimit < 2 || input.MemberLimit > 100 {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_TEAM", "战队资料或人数限制无效")
	}
	var team models.Team
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", identifier, identifier).First(&team).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "TEAM_NOT_FOUND", "战队不存在")
		}
		if err := s.requireManager(tx, team.ID, actorID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.TeamMember{}).Where("team_id=?", team.ID).Count(&count).Error; err != nil {
			return err
		}
		if input.MemberLimit < int(count) {
			return httpx.NewError(http.StatusConflict, "MEMBER_LIMIT_TOO_SMALL", "人数上限不能低于当前成员数")
		}
		return tx.Model(&team).Updates(map[string]any{"name": strings.TrimSpace(input.Name), "slogan": input.Slogan, "description": input.Description, "flag_asset_key": input.FlagAssetKey, "banner_asset_key": input.BannerAssetKey, "member_limit": input.MemberLimit, "recruiting": input.Recruiting, "updated_at": time.Now().UTC()}).Error
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, team.ID.String())
}
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input CreateInput) (Detail, error) {
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 80 || len(input.Slogan) > 240 || len(input.Description) > 5000 || len(input.FlagAssetKey) > 160 || len(input.BannerAssetKey) > 160 {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_TEAM", "战队名称不能为空")
	}
	if input.MemberLimit < 2 {
		input.MemberLimit = 30
	} else if input.MemberLimit > 100 {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_MEMBER_LIMIT", "战队人数上限必须在 2 到 100 之间")
	}
	teamID := uuid.New()
	slug := stableSlug(input.Name, teamID)
	team := models.Team{ID: teamID, Slug: slug, Name: strings.TrimSpace(input.Name), Slogan: input.Slogan, Description: input.Description, FlagAssetKey: input.FlagAssetKey, BannerAssetKey: input.BannerAssetKey, Recruiting: true, CaptainID: userID, MemberLimit: input.MemberLimit}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.TeamMember{}).Where("user_id=?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return httpx.NewError(http.StatusConflict, "ALREADY_IN_TEAM", "你已经加入其他战队")
		}
		if err := tx.Create(&team).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.NewError(http.StatusConflict, "TEAM_EXISTS", "战队名称已存在")
			}
			return err
		}
		return tx.Create(&models.TeamMember{TeamID: team.ID, UserID: userID, Role: "captain", JoinedAt: time.Now().UTC()}).Error
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, team.ID.String())
}

func (s *Service) UploadAvatar(ctx context.Context, actorID uuid.UUID, identifier string, upload assetmodule.Upload) (Detail, error) {
	var team models.Team
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&team).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusNotFound, "TEAM_NOT_FOUND", "战队不存在")
	}
	if team.CaptainID != actorID {
		return Detail{}, httpx.NewError(http.StatusForbidden, "TEAM_CAPTAIN_REQUIRED", "只有队长可以上传战队头像")
	}
	if s.assets == nil {
		return Detail{}, httpx.NewError(http.StatusServiceUnavailable, "ASSET_SERVICE_UNAVAILABLE", "素材服务暂不可用")
	}
	assetKey, err := s.assets.UpsertTeamAvatar(ctx, actorID, team.ID, upload)
	if err != nil {
		return Detail{}, err
	}
	if err := s.db.WithContext(ctx).Model(&team).Updates(map[string]any{"flag_asset_key": assetKey, "updated_at": time.Now().UTC()}).Error; err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, team.ID.String())
}
func stableSlug(name string, id uuid.UUID) string {
	slug := validation.Slug(name)
	if slug == "" {
		return "team-" + id.String()[:8]
	}
	return slug
}
func (s *Service) RequestJoin(ctx context.Context, userID uuid.UUID, identifier, message string) error {
	message = strings.TrimSpace(message)
	if len(message) > 1000 {
		return httpx.NewError(http.StatusBadRequest, "JOIN_REQUEST_MESSAGE_TOO_LONG", "入队申请说明不能超过 1000 个字符")
	}
	var team models.Team
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&team).Error; err != nil {
		return httpx.NewError(http.StatusNotFound, "TEAM_NOT_FOUND", "战队不存在")
	}
	if !team.Recruiting {
		return httpx.NewError(http.StatusConflict, "TEAM_NOT_RECRUITING", "该战队暂未开放招募")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.TeamMember{}).Where("user_id=?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return httpx.NewError(http.StatusConflict, "ALREADY_IN_TEAM", "你已经加入战队")
	}
	request := models.TeamJoinRequest{ID: uuid.New(), TeamID: team.ID, UserID: userID, Message: message, Status: "pending", CreatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Create(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return httpx.NewError(http.StatusConflict, "JOIN_REQUEST_EXISTS", "已有待处理的入队申请")
		}
		return err
	}
	return nil
}
func (s *Service) ReviewJoin(ctx context.Context, actorID, requestID uuid.UUID, approve bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.TeamJoinRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, "id=?", requestID).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "JOIN_REQUEST_NOT_FOUND", "申请不存在")
		}
		if request.Status != "pending" {
			return httpx.NewError(http.StatusConflict, "JOIN_REQUEST_HANDLED", "申请已经处理")
		}
		if err := s.requireManager(tx, request.TeamID, actorID); err != nil {
			return err
		}
		now := time.Now().UTC()
		status := "rejected"
		if approve {
			var team models.Team
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&team, "id=?", request.TeamID).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&models.TeamMember{}).Where("team_id=?", team.ID).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(team.MemberLimit) {
				return httpx.NewError(http.StatusConflict, "TEAM_FULL", "战队成员已满")
			}
			if err := tx.Model(&models.TeamMember{}).Where("user_id=?", request.UserID).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return httpx.NewError(http.StatusConflict, "ALREADY_IN_TEAM", "申请人已经加入其他战队")
			}
			if err := tx.Create(&models.TeamMember{TeamID: team.ID, UserID: request.UserID, Role: "member", JoinedAt: now}).Error; err != nil {
				return err
			}
			status = "approved"
		}
		if err := tx.Model(&request).Updates(map[string]any{"status": status, "reviewed_by": actorID, "reviewed_at": now}).Error; err != nil {
			return err
		}
		title, body := "入队申请未通过", "你的入队申请已被战队管理员拒绝。"
		if status == "approved" {
			title, body = "入队申请已通过", "你已成为战队成员。"
		}
		return tx.Create(&models.Notification{ID: uuid.New(), UserID: request.UserID, Type: "team.join_reviewed", Title: title, Body: body, Link: "/teams/" + request.TeamID.String(), Payload: json.RawMessage(`{}`), CreatedAt: now}).Error
	})
}
func (s *Service) Invite(ctx context.Context, actorID uuid.UUID, teamIdentifier, username string) (uuid.UUID, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 64 {
		return uuid.Nil, httpx.NewError(http.StatusBadRequest, "INVALID_USERNAME", "用户名不合法")
	}
	var team models.Team
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", teamIdentifier, teamIdentifier).First(&team).Error; err != nil {
		return uuid.Nil, httpx.NewError(http.StatusNotFound, "TEAM_NOT_FOUND", "战队不存在")
	}
	if err := s.requireManager(s.db.WithContext(ctx), team.ID, actorID); err != nil {
		return uuid.Nil, err
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("username=?", username).First(&user).Error; err != nil {
		return uuid.Nil, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	}
	id := uuid.New()
	payload, err := json.Marshal(map[string]string{"url": s.webBaseURL + "/team-invitations/" + id.String(), "teamName": team.Name})
	if err != nil {
		return uuid.Nil, err
	}
	ciphertext, err := security.Encrypt(payload, s.confirmationKey)
	if err != nil {
		return uuid.Nil, err
	}
	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("team_invitations").Where("team_id=? AND user_id=? AND status='pending' AND expires_at>?", team.ID, user.ID, now).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return httpx.NewError(http.StatusConflict, "INVITATION_EXISTS", "该用户已有待处理邀请")
		}
		record := map[string]any{"id": id, "team_id": team.ID, "user_id": user.ID, "invited_by": actorID, "status": "pending", "expires_at": now.Add(7 * 24 * time.Hour), "created_at": now}
		if err := tx.Table("team_invitations").Create(record).Error; err != nil {
			return err
		}
		if err := tx.Table("email_outbox").Create(map[string]any{"id": uuid.New(), "recipient": user.Email, "template_key": "team_invitation", "payload_ciphertext": ciphertext}).Error; err != nil {
			return err
		}
		return tx.Create(&models.Notification{ID: uuid.New(), UserID: user.ID, Type: "team.invited", Title: "收到战队邀请", Body: team.Name + " 邀请你加入战队。", Link: "/team-invitations/" + id.String(), Payload: json.RawMessage(`{}`), CreatedAt: now}).Error
	})
	return id, err
}
func (s *Service) AcceptInvitation(ctx context.Context, userID, invitationID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invitation struct {
			ID, TeamID, UserID uuid.UUID
			Status             string
			ExpiresAt          time.Time
		}
		if err := tx.Table("team_invitations").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", invitationID).First(&invitation).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "INVITATION_NOT_FOUND", "邀请不存在")
		}
		if invitation.UserID != userID || invitation.Status != "pending" || invitation.ExpiresAt.Before(time.Now().UTC()) {
			return httpx.NewError(http.StatusConflict, "INVITATION_INVALID", "邀请已失效")
		}
		var team models.Team
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&team, "id=?", invitation.TeamID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.TeamMember{}).Where("team_id=?", team.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(team.MemberLimit) {
			return httpx.NewError(http.StatusConflict, "TEAM_FULL", "战队成员已满")
		}
		if err := tx.Model(&models.TeamMember{}).Where("user_id=?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return httpx.NewError(http.StatusConflict, "ALREADY_IN_TEAM", "你已经加入战队")
		}
		if err := tx.Create(&models.TeamMember{TeamID: invitation.TeamID, UserID: userID, Role: "member", JoinedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Table("team_invitations").Where("id=?", invitationID).Update("status", "accepted").Error
	})
}
func (s *Service) TransferCaptain(ctx context.Context, actorID, teamID, targetID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var team models.Team
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&team, "id=?", teamID).Error; err != nil {
			return err
		}
		if team.CaptainID != actorID {
			return httpx.NewError(http.StatusForbidden, "CAPTAIN_REQUIRED", "仅队长可以转让战队")
		}
		var member models.TeamMember
		if err := tx.First(&member, "team_id=? AND user_id=?", teamID, targetID).Error; err != nil {
			return httpx.NewError(http.StatusBadRequest, "TARGET_NOT_MEMBER", "目标用户不是战队成员")
		}
		if err := tx.Model(&models.TeamMember{}).Where("team_id=? AND user_id=?", teamID, actorID).Update("role", "manager").Error; err != nil {
			return err
		}
		if err := tx.Model(&member).Update("role", "captain").Error; err != nil {
			return err
		}
		return tx.Model(&team).Update("captain_id", targetID).Error
	})
}
func (s *Service) RemoveMember(ctx context.Context, actorID, teamID, targetID uuid.UUID) error {
	if actorID == targetID {
		return httpx.NewError(http.StatusBadRequest, "USE_LEAVE_TEAM", "请使用退出战队操作")
	}
	if err := s.requireManager(s.db.WithContext(ctx), teamID, actorID); err != nil {
		return err
	}
	var team models.Team
	if err := s.db.WithContext(ctx).First(&team, "id=?", teamID).Error; err != nil {
		return err
	}
	if team.CaptainID == targetID {
		return httpx.NewError(http.StatusConflict, "CANNOT_REMOVE_CAPTAIN", "不能移除队长")
	}
	return s.db.WithContext(ctx).Where("team_id=? AND user_id=?", teamID, targetID).Delete(&models.TeamMember{}).Error
}
func (s *Service) Leave(ctx context.Context, userID, teamID uuid.UUID) error {
	var team models.Team
	if err := s.db.WithContext(ctx).First(&team, "id=?", teamID).Error; err != nil {
		return err
	}
	if team.CaptainID == userID {
		return httpx.NewError(http.StatusConflict, "TRANSFER_CAPTAIN_FIRST", "队长退出前必须先转让队长")
	}
	return s.db.WithContext(ctx).Where("team_id=? AND user_id=?", teamID, userID).Delete(&models.TeamMember{}).Error
}
func (s *Service) PostAnnouncement(ctx context.Context, actorID, teamID uuid.UUID, title, content string, pinned bool) error {
	if err := s.requireManager(s.db.WithContext(ctx), teamID, actorID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Table("team_announcements").Create(map[string]any{"id": uuid.New(), "team_id": teamID, "author_id": actorID, "title": title, "content": content, "pinned": pinned, "created_at": time.Now().UTC()}).Error
}
func (s *Service) requireManager(db *gorm.DB, teamID, userID uuid.UUID) error {
	var member models.TeamMember
	if err := db.Where("team_id=? AND user_id=?", teamID, userID).First(&member).Error; err != nil || !(member.Role == "captain" || member.Role == "manager") {
		return httpx.NewError(http.StatusForbidden, "TEAM_MANAGER_REQUIRED", "需要战队管理权限")
	}
	return nil
}
