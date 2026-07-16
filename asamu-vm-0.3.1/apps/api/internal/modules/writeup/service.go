package writeup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/validation"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db       *gorm.DB
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func New(db *gorm.DB) *Service {
	return &Service{db: db, markdown: goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithParserOptions(parser.WithAutoHeadingID())), policy: bluemonday.UGCPolicy()}
}

type View struct {
	ID              uuid.UUID     `json:"id"`
	Slug            string        `json:"slug"`
	Title           string        `json:"title"`
	Summary         string        `json:"summary"`
	ContentHTML     string        `json:"contentHTML"`
	ContentMarkdown string        `json:"contentMarkdown,omitempty"`
	Status          string        `json:"status"`
	Visibility      string        `json:"visibility"`
	Author          string        `json:"author"`
	Category        string        `json:"category"`
	ChallengeTitle  string        `json:"challengeTitle"`
	AuthorID        uuid.UUID     `json:"authorId"`
	ChallengeID     uuid.UUID     `json:"challengeId"`
	CompetitionID   *uuid.UUID    `json:"competitionId,omitempty"`
	Featured        bool          `json:"featured"`
	Views           int64         `json:"views"`
	Likes           int64         `json:"likes"`
	PublishedAt     *time.Time    `json:"publishedAt,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
	RejectReason    string        `json:"rejectReason,omitempty"`
	Liked           bool          `json:"liked"`
	Favorited       bool          `json:"favorited"`
	Comments        []CommentView `json:"comments"`
}
type CommentView struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}
type Mutation struct {
	Slug, Title, Summary, ContentMarkdown, Visibility string
	ChallengeID                                       uuid.UUID
	CompetitionID                                     *uuid.UUID
}

func (s *Service) List(ctx context.Context, category, search string, page, size int, admin bool) (httpx.Page[View], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Table("writeups w").Joins("JOIN users u ON u.id=w.author_id").Joins("JOIN challenges c ON c.id=w.challenge_id").Joins("JOIN challenge_categories cat ON cat.id=c.category_id")
	if !admin {
		query = query.Where("w.status='published' AND w.visibility='public' AND (w.opens_at IS NULL OR w.opens_at<=?)", time.Now().UTC())
	}
	if category != "" {
		query = query.Where("cat.key=? OR cat.name=?", category, category)
	}
	if search != "" {
		query = query.Where("w.title ILIKE ? OR w.summary ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	var items []View
	selectSQL := `w.id,w.slug,w.title,w.summary,w.content_html,w.status,w.visibility,u.username AS author,w.author_id,cat.name AS category,c.title AS challenge_title,w.challenge_id,w.competition_id,w.featured,w.views,w.likes,w.published_at,w.created_at,w.updated_at,w.reject_reason`
	if err := query.Select(selectSQL).Order("w.featured DESC,w.published_at DESC,w.created_at DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	return httpx.Page[View]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Detail(ctx context.Context, identifier string, viewer *uuid.UUID, admin bool) (View, error) {
	var item View
	query := s.db.WithContext(ctx).Table("writeups w").Select(`w.id,w.slug,w.title,w.summary,w.content_markdown,w.content_html,w.status,w.visibility,u.username AS author,w.author_id,cat.name AS category,c.title AS challenge_title,w.challenge_id,w.competition_id,w.featured,w.views,w.likes,w.published_at,w.created_at,w.updated_at,w.reject_reason`).Joins("JOIN users u ON u.id=w.author_id").Joins("JOIN challenges c ON c.id=w.challenge_id").Joins("JOIN challenge_categories cat ON cat.id=c.category_id").Where("w.id::text=? OR w.slug=?", identifier, identifier)
	if !admin {
		query = query.Where("w.status='published' AND (w.opens_at IS NULL OR w.opens_at<=?)", time.Now().UTC())
		if viewer == nil {
			query = query.Where("w.visibility IN ('public','unlisted')")
		} else {
			query = query.Where("w.visibility IN ('public','unlisted') OR w.author_id=?", *viewer)
		}
	}
	if err := query.First(&item).Error; err != nil {
		return View{}, httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在或尚未开放")
	}
	if !admin {
		_ = s.db.WithContext(ctx).Table("writeups").Where("id=?", item.ID).Update("views", gorm.Expr("views+1")).Error
		item.Views++
	}
	item.Comments = []CommentView{}
	_ = s.db.WithContext(ctx).Table("writeup_comments wc").Select("wc.id,wc.user_id,u.username,wc.content,wc.created_at").Joins("JOIN users u ON u.id=wc.user_id").Where("wc.writeup_id=? AND wc.status='visible'", item.ID).Order("wc.created_at").Limit(100).Scan(&item.Comments).Error
	if viewer != nil {
		var count int64
		_ = s.db.WithContext(ctx).Table("writeup_likes").Where("writeup_id=? AND user_id=?", item.ID, *viewer).Count(&count).Error
		item.Liked = count != 0
		count = 0
		_ = s.db.WithContext(ctx).Table("writeup_favorites").Where("writeup_id=? AND user_id=?", item.ID, *viewer).Count(&count).Error
		item.Favorited = count != 0
	}
	return item, nil
}
func (s *Service) Mine(ctx context.Context, userID uuid.UUID, page, size int) (httpx.Page[View], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	base := s.db.WithContext(ctx).Table("writeups w").Where("w.author_id=?", userID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	var items []View
	selectSQL := `w.id,w.slug,w.title,w.summary,w.content_markdown,w.content_html,w.status,w.visibility,u.username AS author,w.author_id,cat.name AS category,c.title AS challenge_title,w.challenge_id,w.competition_id,w.featured,w.views,w.likes,w.published_at,w.created_at,w.updated_at,w.reject_reason`
	err := base.Select(selectSQL).Joins("JOIN users u ON u.id=w.author_id").Joins("JOIN challenges c ON c.id=w.challenge_id").Joins("JOIN challenge_categories cat ON cat.id=c.category_id").Order("w.updated_at DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error
	return httpx.Page[View]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, err
}
func (s *Service) MineDetail(ctx context.Context, userID uuid.UUID, identifier string) (View, error) {
	item, err := s.Detail(ctx, identifier, &userID, true)
	if err != nil {
		return View{}, err
	}
	if item.AuthorID != userID {
		return View{}, httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
	}
	return item, nil
}
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input Mutation) (View, error) {
	if err := validateMutation(input); err != nil {
		return View{}, err
	}
	if input.Title == "" || input.ChallengeID == uuid.Nil {
		return View{}, httpx.NewError(http.StatusBadRequest, "INVALID_WRITEUP", "标题和关联题目不能为空")
	}
	if input.Slug == "" {
		input.Slug = validation.Slug(input.Title)
	}
	html, err := s.render(input.ContentMarkdown)
	if err != nil {
		return View{}, err
	}
	writeup := models.Writeup{ID: uuid.New(), Slug: input.Slug, AuthorID: userID, ChallengeID: input.ChallengeID, CompetitionID: input.CompetitionID, Title: input.Title, Summary: input.Summary, ContentMarkdown: input.ContentMarkdown, ContentHTML: html, Status: "draft", Visibility: input.Visibility, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if writeup.Visibility == "" {
		writeup.Visibility = "public"
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&writeup).Error; err != nil {
			return err
		}
		return tx.Table("writeup_revisions").Create(map[string]any{"id": uuid.New(), "writeup_id": writeup.ID, "version": 1, "content_markdown": input.ContentMarkdown, "created_by": userID, "created_at": time.Now().UTC()}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return View{}, httpx.NewError(http.StatusConflict, "WRITEUP_SLUG_EXISTS", "WriteUp Slug 已存在，请换一个")
		}
		return View{}, err
	}
	return s.Detail(ctx, writeup.ID.String(), &userID, true)
}
func (s *Service) Update(ctx context.Context, userID uuid.UUID, identifier string, input Mutation) (View, error) {
	if err := validateMutation(input); err != nil {
		return View{}, err
	}
	var writeup models.Writeup
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&writeup).Error; err != nil {
		return View{}, httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
	}
	if writeup.AuthorID != userID {
		return View{}, httpx.NewError(http.StatusForbidden, "WRITEUP_OWNER_REQUIRED", "只能编辑自己的 WriteUp")
	}
	if writeup.Status == "published" || writeup.Status == "archived" {
		return View{}, httpx.NewError(http.StatusConflict, "WRITEUP_LOCKED", "已发布内容需创建新修订")
	}
	html, err := s.render(input.ContentMarkdown)
	if err != nil {
		return View{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&writeup).Updates(map[string]any{"title": input.Title, "summary": input.Summary, "content_markdown": input.ContentMarkdown, "content_html": html, "visibility": input.Visibility, "challenge_id": input.ChallengeID, "competition_id": input.CompetitionID, "status": "draft", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		var version int64
		if err := tx.Table("writeup_revisions").Where("writeup_id=?", writeup.ID).Count(&version).Error; err != nil {
			return err
		}
		return tx.Table("writeup_revisions").Create(map[string]any{"id": uuid.New(), "writeup_id": writeup.ID, "version": version + 1, "content_markdown": input.ContentMarkdown, "created_by": userID, "created_at": time.Now().UTC()}).Error
	})
	if err != nil {
		return View{}, err
	}
	return s.Detail(ctx, writeup.ID.String(), &userID, true)
}
func validateMutation(input Mutation) error {
	if strings.TrimSpace(input.Title) == "" || input.ChallengeID == uuid.Nil {
		return httpx.NewError(http.StatusBadRequest, "INVALID_WRITEUP", "标题和关联题目不能为空")
	}
	if len(input.Title) > 160 || len(input.Summary) > 1000 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_WRITEUP", "标题或摘要过长")
	}
	if input.Visibility != "" && input.Visibility != "public" && input.Visibility != "unlisted" && input.Visibility != "private" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_VISIBILITY", "可见范围无效")
	}
	return nil
}
func (s *Service) SubmitReview(ctx context.Context, userID uuid.UUID, identifier string) error {
	result := s.db.WithContext(ctx).Model(&models.Writeup{}).Where("(id::text=? OR slug=?) AND author_id=? AND status IN ('draft','rejected')", identifier, identifier, userID).Updates(map[string]any{"status": "review", "reject_reason": "", "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return httpx.NewError(http.StatusConflict, "WRITEUP_NOT_SUBMITTABLE", "WriteUp 当前不能提交审核")
	}
	return nil
}
func (s *Service) Review(ctx context.Context, reviewer uuid.UUID, identifier, action, note string) error {
	if action != "publish" && action != "reject" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_REVIEW_ACTION", "审核动作无效")
	}
	if len(note) > 2000 || (action == "reject" && strings.TrimSpace(note) == "") {
		return httpx.NewError(http.StatusBadRequest, "INVALID_REVIEW_NOTE", "驳回时必须填写不超过 2000 字的审核意见")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var writeup models.Writeup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", identifier, identifier).First(&writeup).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
		}
		if writeup.Status != "review" {
			return httpx.NewError(http.StatusConflict, "WRITEUP_NOT_IN_REVIEW", "WriteUp 不在审核队列")
		}
		updates := map[string]any{"updated_at": time.Now().UTC()}
		if action == "publish" {
			now := time.Now().UTC()
			updates["status"] = "published"
			updates["published_at"] = now
			updates["reject_reason"] = ""
		} else {
			updates["status"] = "rejected"
			updates["reject_reason"] = note
		}
		if err := tx.Model(&writeup).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Table("writeup_review_logs").Create(map[string]any{"id": uuid.New(), "writeup_id": writeup.ID, "reviewer_id": reviewer, "action": action, "note": note, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		title, body, link := "WriteUp 已发布", "你的文章《"+writeup.Title+"》已通过审核。", "/writeups/"+writeup.Slug
		if action == "reject" {
			title, body, link = "WriteUp 需要修改", "你的文章《"+writeup.Title+"》未通过审核："+note, "/writeups/"+writeup.ID.String()+"/edit"
		}
		return tx.Create(&models.Notification{ID: uuid.New(), UserID: writeup.AuthorID, Type: "writeup.reviewed", Title: title, Body: body, Link: link, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}).Error
	})
}
func (s *Service) Comment(ctx context.Context, userID uuid.UUID, identifier, content string) (models.WriteupComment, error) {
	if err := s.ensureInteractive(ctx, identifier); err != nil {
		return models.WriteupComment{}, err
	}
	if len(content) < 2 || len(content) > 2000 {
		return models.WriteupComment{}, httpx.NewError(http.StatusBadRequest, "INVALID_COMMENT", "评论长度不合法")
	}
	var writeup models.Writeup
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&writeup).Error; err != nil {
		return models.WriteupComment{}, httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
	}
	comment := models.WriteupComment{ID: uuid.New(), WriteupID: writeup.ID, UserID: userID, Content: s.policy.Sanitize(content), Status: "visible", CreatedAt: time.Now().UTC()}
	return comment, s.db.WithContext(ctx).Create(&comment).Error
}
func (s *Service) ToggleLike(ctx context.Context, userID uuid.UUID, identifier string) (bool, error) {
	if err := s.ensureInteractive(ctx, identifier); err != nil {
		return false, err
	}
	var writeup models.Writeup
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&writeup).Error; err != nil {
		return false, httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
	}
	liked := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("writeup_likes").Where("writeup_id=? AND user_id=?", writeup.ID, userID).Delete(nil)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return tx.Model(&writeup).Update("likes", gorm.Expr("GREATEST(likes-1,0)")).Error
		}
		liked = true
		if err := tx.Table("writeup_likes").Create(map[string]any{"writeup_id": writeup.ID, "user_id": userID, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Model(&writeup).Update("likes", gorm.Expr("likes+1")).Error
	})
	return liked, err
}
func (s *Service) Favorite(ctx context.Context, userID uuid.UUID, identifier string, enabled bool) error {
	if err := s.ensureInteractive(ctx, identifier); err != nil {
		return err
	}
	var writeup models.Writeup
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", identifier, identifier).First(&writeup).Error; err != nil {
		return httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在")
	}
	if !enabled {
		return s.db.WithContext(ctx).Table("writeup_favorites").Where("writeup_id=? AND user_id=?", writeup.ID, userID).Delete(nil).Error
	}
	return s.db.WithContext(ctx).Table("writeup_favorites").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"writeup_id": writeup.ID, "user_id": userID, "created_at": time.Now().UTC()}).Error
}
func (s *Service) ensureInteractive(ctx context.Context, identifier string) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Writeup{}).Where("(id::text=? OR slug=?) AND status='published' AND visibility IN ('public','unlisted') AND (opens_at IS NULL OR opens_at<=?)", identifier, identifier, time.Now().UTC()).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return httpx.NewError(http.StatusNotFound, "WRITEUP_NOT_FOUND", "WriteUp 不存在或尚未开放")
	}
	return nil
}
func (s *Service) render(markdown string) (string, error) {
	if len(markdown) > 500_000 {
		return "", httpx.NewError(http.StatusBadRequest, "WRITEUP_TOO_LARGE", "WriteUp 内容过长")
	}
	var buffer bytes.Buffer
	if err := s.markdown.Convert([]byte(markdown), &buffer); err != nil {
		return "", err
	}
	return s.policy.Sanitize(buffer.String()), nil
}

var _ = errors.Is
