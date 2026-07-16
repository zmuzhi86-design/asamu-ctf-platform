package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	learningmodule "asamu.local/platform/api/internal/modules/learning"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/security"
	"asamu.local/platform/api/internal/platform/validation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db         *gorm.DB
	flagSecret []byte
}

func New(db *gorm.DB, flagSecret string) *Service {
	return &Service{db: db, flagSecret: []byte(flagSecret)}
}

type Filter struct {
	Search, Category, Difficulty, Status string
	Dynamic                              *bool
	Page, PageSize                       int
}
type View struct {
	ID           uuid.UUID  `json:"id"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Category     string     `json:"category"`
	CategoryKey  string     `json:"categoryKey"`
	Difficulty   string     `json:"difficulty"`
	Summary      string     `json:"summary"`
	Description  string     `json:"description"`
	Author       string     `json:"author"`
	Score        int        `json:"score"`
	MinimumScore int        `json:"minimumScore"`
	MaximumScore int        `json:"maximumScore"`
	DynamicDecay int        `json:"dynamicDecay"`
	ScoreMode    string     `json:"scoreMode"`
	Visibility   string     `json:"visibility"`
	Solves       int64      `json:"solves"`
	Attempts     int64      `json:"attempts"`
	SolveRate    float64    `json:"solveRate"`
	Dynamic      bool       `json:"dynamic"`
	Attachment   bool       `json:"attachment"`
	Writeup      bool       `json:"writeup"`
	Tags         []string   `json:"tags"`
	Status       string     `json:"status"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
}
type Detail struct {
	View
	Hints           []Hint   `json:"hints"`
	Files           []File   `json:"files"`
	KnowledgePoints []string `json:"knowledgePoints"`
	Runtime         *Runtime `json:"runtime,omitempty"`
	Similar         []View   `json:"similar"`
	Bloods          []Blood  `json:"bloods"`
}
type Hint struct {
	ID      uuid.UUID `json:"id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Cost    int       `json:"cost"`
}
type File struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MIMEType    string    `json:"mimeType"`
	Size        int64     `json:"size"`
	DownloadURL string    `json:"downloadUrl"`
}
type Runtime struct {
	RegistryCredentialID *uuid.UUID        `json:"registryCredentialId,omitempty"`
	ImageRef             string            `json:"imageRef,omitempty"`
	ImageDigest          string            `json:"imageDigest,omitempty"`
	InternalPort         int               `json:"internalPort"`
	CPUMilli             int               `json:"cpuMilli"`
	MemoryMB             int               `json:"memoryMB"`
	TTLSeconds           int               `json:"ttlSeconds"`
	MaxTTLSeconds        int               `json:"maxTTLSeconds"`
	Protocol             string            `json:"protocol"`
	FlagFormat           string            `json:"flagFormat"`
	PIDsLimit            int               `json:"pidsLimit,omitempty"`
	DiskMB               int               `json:"diskMB,omitempty"`
	ReadOnlyRootFS       bool              `json:"readOnlyRootFS,omitempty"`
	Environment          map[string]string `gorm:"-" json:"environment,omitempty"`
	EnvironmentTemplate  json.RawMessage   `json:"-"`
}
type Blood struct {
	Rank      int       `json:"rank"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
}
type Mutation struct {
	Slug, Title, CategoryKey, Difficulty, Summary, Description, Author, ScoreMode, Visibility string
	BaseScore, MinimumScore, MaximumScore, DynamicDecay                                       int
	IsDynamic                                                                                 bool
	Tags, Hints, KnowledgePoints                                                              []string
	HintConfigs                                                                               []HintMutation `json:"hintConfigs"`
	Flags                                                                                     []FlagMutation `json:"flags"`
	Runtime                                                                                   *RuntimeMutation
}
type HintMutation struct {
	Title, Content string
	Cost           int
}
type FlagMutation struct {
	Kind, Value string
	Stage       int
}
type RuntimeMutation struct {
	RegistryCredentialID                                                           *uuid.UUID `json:"registryCredentialId"`
	ImageRef, ImageDigest, Protocol, FlagFormat                                    string
	InternalPort, CPUMilli, MemoryMB, PIDsLimit, DiskMB, TTLSeconds, MaxTTLSeconds int
	ReadOnlyRootFS                                                                 bool
	Environment                                                                    map[string]string `json:"environment"`
}

func (s *Service) List(ctx context.Context, filter Filter, includeDraft bool) (httpx.Page[View], error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	query := s.db.WithContext(ctx).Table("challenges c").Joins("JOIN challenge_categories cc ON cc.id = c.category_id")
	if !includeDraft {
		query = query.Where("c.status = 'published'")
	} else if filter.Status != "" {
		query = query.Where("c.status = ?", filter.Status)
	}
	if filter.Search != "" {
		term := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("c.title ILIKE ? OR c.summary ILIKE ?", term, term)
	}
	if filter.Category != "" {
		query = query.Where("cc.key = ? OR cc.name = ?", filter.Category, filter.Category)
	}
	if filter.Difficulty != "" {
		query = query.Where("c.difficulty = ?", filter.Difficulty)
	}
	if filter.Dynamic != nil {
		query = query.Where("c.is_dynamic = ?", *filter.Dynamic)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	rows := []View{}
	selectSQL := `c.id,c.slug,c.title,cc.name AS category,cc.key AS category_key,c.difficulty,c.summary,c.description_markdown AS description,c.author_name AS author,c.base_score AS score,c.minimum_score,c.maximum_score,c.dynamic_decay,c.score_mode,c.visibility,c.solve_count AS solves,c.attempt_count AS attempts,CASE WHEN c.attempt_count=0 THEN 0 ELSE round(c.solve_count::numeric*100/c.attempt_count,2) END AS solve_rate,c.is_dynamic AS dynamic,c.has_attachment AS attachment,c.has_writeup AS writeup,c.status,c.published_at`
	if err := query.Select(selectSQL).Order("cc.sort_order,c.base_score,c.title").Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&rows).Error; err != nil {
		return httpx.Page[View]{}, err
	}
	if err := s.loadTags(ctx, rows); err != nil {
		return httpx.Page[View]{}, err
	}
	pages := int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	return httpx.Page[View]{Items: rows, Page: filter.Page, PageSize: filter.PageSize, Total: total, TotalPages: pages}, nil
}
func (s *Service) loadTags(ctx context.Context, rows []View) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	index := map[uuid.UUID]int{}
	for i, row := range rows {
		ids[i] = row.ID
		index[row.ID] = i
		rows[i].Tags = []string{}
	}
	var links []struct {
		ChallengeID uuid.UUID
		Name        string
	}
	if err := s.db.WithContext(ctx).Table("challenge_tag_links ctl").Select("ctl.challenge_id,ct.name").Joins("JOIN challenge_tags ct ON ct.id=ctl.tag_id").Where("ctl.challenge_id IN ?", ids).Scan(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		rows[index[link.ChallengeID]].Tags = append(rows[index[link.ChallengeID]].Tags, link.Name)
	}
	return nil
}
func (s *Service) Detail(ctx context.Context, identifier string, includeDraft bool) (Detail, error) {
	query := s.db.WithContext(ctx).Table("challenges c").Joins("JOIN challenge_categories cc ON cc.id=c.category_id")
	if !includeDraft {
		query = query.Where("c.status='published'")
	}
	var view View
	if err := query.Select(`c.id,c.slug,c.title,cc.name AS category,cc.key AS category_key,c.difficulty,c.summary,c.description_markdown AS description,c.author_name AS author,c.base_score AS score,c.minimum_score,c.maximum_score,c.dynamic_decay,c.score_mode,c.visibility,c.solve_count AS solves,c.attempt_count AS attempts,CASE WHEN c.attempt_count=0 THEN 0 ELSE round(c.solve_count::numeric*100/c.attempt_count,2) END AS solve_rate,c.is_dynamic AS dynamic,c.has_attachment AS attachment,c.has_writeup AS writeup,c.status,c.published_at`).Where("c.id::text=? OR c.slug=?", identifier, identifier).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Detail{}, httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
		}
		return Detail{}, err
	}
	views := []View{view}
	if err := s.loadTags(ctx, views); err != nil {
		return Detail{}, err
	}
	view = views[0]
	detail := Detail{View: view, Hints: []Hint{}, Files: []File{}, KnowledgePoints: []string{}, Similar: []View{}, Bloods: []Blood{}}
	if includeDraft {
		_ = s.db.WithContext(ctx).Table("challenge_hints").Select("id,title,content_markdown AS content,cost").Where("challenge_id=?", view.ID).Order("sort_order").Scan(&detail.Hints).Error
	} else {
		var raw json.RawMessage
		if err := s.db.WithContext(ctx).Table("challenge_revisions r").Select("r.hints_json").Joins("JOIN challenges c ON c.current_published_revision_id=r.id").Where("c.id=?", view.ID).Scan(&raw).Error; err == nil {
			var frozen []struct {
				Title string `json:"title"`
				Cost  int    `json:"cost"`
			}
			if json.Unmarshal(raw, &frozen) == nil {
				for _, item := range frozen {
					detail.Hints = append(detail.Hints, Hint{Title: item.Title, Cost: item.Cost})
				}
			}
		}
	}
	var files []struct {
		ID             uuid.UUID
		Name, MIMEType string
		Size           int64
	}
	if includeDraft {
		_ = s.db.WithContext(ctx).Table("challenge_files").Select("id,name,mime_type,size").Where("challenge_id=? AND archived_at IS NULL", view.ID).Scan(&files).Error
	} else {
		_ = s.db.WithContext(ctx).Table("challenge_file_revisions r").Select("r.source_file_id AS id,r.name,r.mime_type,r.size").Joins("JOIN challenges c ON c.current_published_revision_id=r.challenge_revision_id").Where("c.id=? AND r.source_file_id IS NOT NULL", view.ID).Scan(&files).Error
	}
	for _, file := range files {
		detail.Files = append(detail.Files, File{ID: file.ID, Name: file.Name, MIMEType: file.MIMEType, Size: file.Size, DownloadURL: "/api/v1/challenges/" + view.Slug + "/files/" + file.ID.String()})
	}
	_ = s.db.WithContext(ctx).Table("challenge_knowledge_points").Where("challenge_id=?", view.ID).Order("sort_order").Pluck("name", &detail.KnowledgePoints).Error
	if view.Dynamic {
		var runtime Runtime
		columns := "internal_port,cpu_milli,memory_mb,ttl_seconds,max_ttl_seconds,protocol,flag_format"
		if includeDraft {
			columns += ",image_ref,image_digest,pids_limit,disk_mb,read_only_root_fs,registry_credential_id,environment_template"
		}
		if err := s.db.WithContext(ctx).Table("challenge_runtime_configs").Select(columns).Where("challenge_id=? AND enabled=true", view.ID).First(&runtime).Error; err == nil {
			if includeDraft && len(runtime.EnvironmentTemplate) > 0 {
				_ = json.Unmarshal(runtime.EnvironmentTemplate, &runtime.Environment)
			}
			detail.Runtime = &runtime
		}
	}
	_ = s.db.WithContext(ctx).Table("blood_records br").Select("br.rank,u.username,br.created_at").Joins("JOIN users u ON u.id=br.user_id").Where("br.challenge_id=? AND br.competition_id IS NULL", view.ID).Order("br.rank").Limit(3).Scan(&detail.Bloods).Error
	return detail, nil
}
func (s *Service) Create(ctx context.Context, input Mutation) (Detail, error) {
	if err := normalizeMutation(&input); err != nil {
		return Detail{}, err
	}
	challengeID := uuid.New()
	if input.Slug == "" {
		input.Slug = validation.Slug(input.Title)
	}
	input.Slug = validation.Slug(input.Slug)
	if input.Slug == "" {
		input.Slug = "challenge-" + strings.ReplaceAll(challengeID.String(), "-", "")[:10]
	}
	var slugCount int64
	if err := s.db.WithContext(ctx).Model(&models.Challenge{}).Where("slug=?", input.Slug).Count(&slugCount).Error; err != nil {
		return Detail{}, err
	}
	if slugCount > 0 {
		return Detail{}, httpx.NewError(http.StatusConflict, "CHALLENGE_SLUG_EXISTS", "题目 Slug 已存在，请换一个")
	}
	var category models.ChallengeCategory
	if err := s.db.WithContext(ctx).Where("key=? AND enabled=true", input.CategoryKey).First(&category).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_CATEGORY", "题目方向不存在")
	}
	directionID := category.ID
	challenge := models.Challenge{ID: challengeID, Slug: input.Slug, CategoryID: category.ID, DirectionID: &directionID, Title: input.Title, Difficulty: input.Difficulty, Summary: input.Summary, DescriptionMarkdown: input.Description, AuthorName: input.Author, Status: "draft", Visibility: input.Visibility, ScoreMode: input.ScoreMode, BaseScore: input.BaseScore, MinimumScore: input.MinimumScore, MaximumScore: input.MaximumScore, DynamicDecay: input.DynamicDecay, IsDynamic: input.IsDynamic}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&challenge).Error; err != nil {
			return err
		}
		return s.replaceRelations(tx, &challenge, input)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, challenge.ID.String(), true)
}
func (s *Service) Update(ctx context.Context, id string, input Mutation) (Detail, error) {
	if err := normalizeMutation(&input); err != nil {
		return Detail{}, err
	}
	var challenge models.Challenge
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", id, id).First(&challenge).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
	}
	var category models.ChallengeCategory
	if err := s.db.WithContext(ctx).Where("key=? AND enabled=true", input.CategoryKey).First(&category).Error; err != nil {
		return Detail{}, httpx.NewError(http.StatusBadRequest, "INVALID_CATEGORY", "题目方向不存在")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"category_id": category.ID, "direction_id": category.ID, "title": input.Title, "difficulty": input.Difficulty, "summary": input.Summary, "description_markdown": input.Description, "author_name": input.Author, "visibility": input.Visibility, "score_mode": input.ScoreMode, "base_score": input.BaseScore, "minimum_score": input.MinimumScore, "maximum_score": input.MaximumScore, "dynamic_decay": input.DynamicDecay, "is_dynamic": input.IsDynamic, "updated_at": time.Now().UTC()}
		if err := tx.Model(&challenge).Updates(updates).Error; err != nil {
			return err
		}
		return s.replaceRelations(tx, &challenge, input)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Detail(ctx, challenge.ID.String(), true)
}
func (s *Service) replaceRelations(tx *gorm.DB, challenge *models.Challenge, input Mutation) error {
	if err := tx.Where("challenge_id=?", challenge.ID).Delete(&models.ChallengeTagLink{}).Error; err != nil {
		return err
	}
	for _, name := range input.Tags {
		candidate := models.ChallengeTag{ID: uuid.New(), Name: name}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
		var tag models.ChallengeTag
		if err := tx.Where("name=?", name).Take(&tag).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ChallengeTagLink{ChallengeID: challenge.ID, TagID: tag.ID}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("challenge_id=?", challenge.ID).Delete(&models.ChallengeHint{}).Error; err != nil {
		return err
	}
	hints := input.HintConfigs
	if len(hints) == 0 {
		for index, content := range input.Hints {
			hints = append(hints, HintMutation{Title: "Hint " + strconv.Itoa(index+1), Content: content})
		}
	}
	for index, hint := range hints {
		if hint.Cost < 0 {
			return httpx.NewError(http.StatusBadRequest, "INVALID_HINT_COST", "Hint 扣分不能为负数")
		}
		if hint.Title == "" {
			hint.Title = "Hint " + strconv.Itoa(index+1)
		}
		if err := tx.Create(&models.ChallengeHint{ID: uuid.New(), ChallengeID: challenge.ID, Title: hint.Title, ContentMarkdown: hint.Content, Cost: hint.Cost, SortOrder: index}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("challenge_id=?", challenge.ID).Delete(&models.ChallengeKnowledgePoint{}).Error; err != nil {
		return err
	}
	for index, name := range input.KnowledgePoints {
		if err := tx.Create(&models.ChallengeKnowledgePoint{ID: uuid.New(), ChallengeID: challenge.ID, Name: name, SortOrder: index}).Error; err != nil {
			return err
		}
	}
	if input.Flags != nil {
		if err := tx.Where("challenge_id=?", challenge.ID).Delete(&models.ChallengeFlag{}).Error; err != nil {
			return err
		}
		for index, flag := range input.Flags {
			if flag.Stage < 1 {
				flag.Stage = index + 1
			}
			record := models.ChallengeFlag{ID: uuid.New(), ChallengeID: challenge.ID, Kind: flag.Kind, Stage: flag.Stage, Enabled: true, CreatedAt: time.Now().UTC()}
			switch flag.Kind {
			case "static", "multi_stage":
				value := strings.TrimSpace(flag.Value)
				if len(value) < 6 || len(value) > 512 {
					return httpx.NewError(http.StatusBadRequest, "INVALID_FLAG", "Flag 长度需要在 6 到 512 个字符之间")
				}
				record.HMAC = security.FlagHMAC(s.flagSecret, value)
			case "regex":
				if len(flag.Value) == 0 || len(flag.Value) > 512 {
					return httpx.NewError(http.StatusBadRequest, "INVALID_FLAG_REGEX", "Flag 正则长度不合法")
				}
				if _, err := regexp.Compile(flag.Value); err != nil {
					return httpx.NewError(http.StatusBadRequest, "INVALID_FLAG_REGEX", "Flag 正则无法编译")
				}
				record.RegexPattern = flag.Value
			default:
				return httpx.NewError(http.StatusBadRequest, "INVALID_FLAG_KIND", "Flag 类型仅支持 static、multi_stage 或 regex")
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
	}
	if input.IsDynamic && input.Runtime != nil {
		if err := validateRuntime(input.Runtime); err != nil {
			return err
		}
		if input.Runtime.RegistryCredentialID != nil {
			var credential models.RegistryCredential
			if err := tx.Where("id=? AND enabled=true", *input.Runtime.RegistryCredentialID).First(&credential).Error; err != nil {
				return httpx.NewError(http.StatusBadRequest, "REGISTRY_CREDENTIAL_NOT_FOUND", "镜像仓库凭据不存在或已停用")
			}
			if imageRegistryHost(input.Runtime.ImageRef) != credential.RegistryHost {
				return httpx.NewError(http.StatusBadRequest, "REGISTRY_CREDENTIAL_HOST_MISMATCH", "镜像引用与所选仓库凭据不匹配")
			}
		}
		environmentJSON, _ := json.Marshal(input.Runtime.Environment)
		runtime := models.ChallengeRuntimeConfig{ID: uuid.New(), ChallengeID: challenge.ID, RegistryCredentialID: input.Runtime.RegistryCredentialID, ImageRef: strings.TrimSpace(input.Runtime.ImageRef), ImageDigest: strings.TrimSpace(input.Runtime.ImageDigest), InternalPort: input.Runtime.InternalPort, Protocol: input.Runtime.Protocol, FlagFormat: input.Runtime.FlagFormat, CPUMilli: input.Runtime.CPUMilli, MemoryMB: input.Runtime.MemoryMB, PIDsLimit: input.Runtime.PIDsLimit, DiskMB: input.Runtime.DiskMB, TTLSeconds: input.Runtime.TTLSeconds, MaxTTLSeconds: input.Runtime.MaxTTLSeconds, ReadOnlyRootFS: input.Runtime.ReadOnlyRootFS, EnvironmentTemplate: environmentJSON, Enabled: true}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "challenge_id"}}, DoUpdates: clause.AssignmentColumns([]string{"registry_credential_id", "image_ref", "image_digest", "internal_port", "protocol", "flag_format", "cpu_milli", "memory_mb", "pids_limit", "disk_mb", "ttl_seconds", "max_ttl_seconds", "read_only_root_fs", "environment_template", "enabled", "updated_at"})}).Create(&runtime).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Publish(ctx context.Context, id string, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var challenge models.Challenge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", id, id).First(&challenge).Error; err != nil {
			return httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
		}
		var version int
		if err := tx.Table("challenge_revisions").Where("challenge_id=?", challenge.ID).Select("COALESCE(MAX(version),0)+1").Scan(&version).Error; err != nil {
			return err
		}
		var tags []string
		if err := tx.Table("challenge_tag_links l").Joins("JOIN challenge_tags t ON t.id=l.tag_id").Where("l.challenge_id=?", challenge.ID).Order("t.name").Pluck("t.name", &tags).Error; err != nil {
			return err
		}
		var hints []map[string]any
		if err := tx.Table("challenge_hints").Select("title,content_markdown AS content,cost,sort_order").Where("challenge_id=?", challenge.ID).Order("sort_order").Find(&hints).Error; err != nil {
			return err
		}
		var knowledge []string
		if err := tx.Table("challenge_knowledge_points").Where("challenge_id=?", challenge.ID).Order("sort_order").Pluck("name", &knowledge).Error; err != nil {
			return err
		}
		tagsJSON, _ := json.Marshal(tags)
		hintsJSON, _ := json.Marshal(hints)
		knowledgeJSON, _ := json.Marshal(knowledge)
		now := time.Now().UTC()
		publishedBy := actor
		revision := models.ChallengeRevision{ID: uuid.New(), ChallengeID: challenge.ID, CategoryID: challenge.CategoryID, DirectionID: challenge.DirectionID, Version: version, Title: challenge.Title, Summary: challenge.Summary, DescriptionMarkdown: challenge.DescriptionMarkdown, Difficulty: challenge.Difficulty, AuthorName: challenge.AuthorName, Visibility: challenge.Visibility, ScoreMode: challenge.ScoreMode, BaseScore: challenge.BaseScore, MinimumScore: challenge.MinimumScore, MaximumScore: challenge.MaximumScore, DynamicDecay: challenge.DynamicDecay, IsDynamic: challenge.IsDynamic, TagsJSON: tagsJSON, HintsJSON: hintsJSON, KnowledgePointsJSON: knowledgeJSON, PublishedBy: &publishedBy, PublishedAt: now, CreatedAt: now}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if challenge.IsDynamic {
			var runtimeCfg models.ChallengeRuntimeConfig
			if err := tx.Where("challenge_id=? AND enabled=true", challenge.ID).First(&runtimeCfg).Error; err != nil {
				return httpx.NewError(http.StatusUnprocessableEntity, "RUNTIME_NOT_CONFIGURED", "动态题目必须先配置运行环境")
			}
			var environment map[string]string
			_ = json.Unmarshal(runtimeCfg.EnvironmentTemplate, &environment)
			if err := validateRuntime(&RuntimeMutation{RegistryCredentialID: runtimeCfg.RegistryCredentialID, ImageRef: runtimeCfg.ImageRef, ImageDigest: runtimeCfg.ImageDigest, InternalPort: runtimeCfg.InternalPort, Protocol: runtimeCfg.Protocol, FlagFormat: runtimeCfg.FlagFormat, CPUMilli: runtimeCfg.CPUMilli, MemoryMB: runtimeCfg.MemoryMB, PIDsLimit: runtimeCfg.PIDsLimit, DiskMB: runtimeCfg.DiskMB, TTLSeconds: runtimeCfg.TTLSeconds, MaxTTLSeconds: runtimeCfg.MaxTTLSeconds, ReadOnlyRootFS: runtimeCfg.ReadOnlyRootFS, Environment: environment}); err != nil {
				return httpx.NewError(http.StatusUnprocessableEntity, "INVALID_RUNTIME_CONFIG", err.Error())
			}
			if runtimeCfg.RegistryCredentialID != nil {
				var credential models.RegistryCredential
				if err := tx.Where("id=? AND enabled=true", *runtimeCfg.RegistryCredentialID).First(&credential).Error; err != nil {
					return httpx.NewError(http.StatusUnprocessableEntity, "REGISTRY_CREDENTIAL_NOT_FOUND", "镜像仓库凭据不存在或已停用")
				}
				if imageRegistryHost(runtimeCfg.ImageRef) != credential.RegistryHost {
					return httpx.NewError(http.StatusUnprocessableEntity, "REGISTRY_CREDENTIAL_HOST_MISMATCH", "镜像引用与所选仓库凭据不匹配")
				}
			}
			runtimeRevision := models.ChallengeRuntimeRevision{ID: uuid.New(), ChallengeRevisionID: revision.ID, RegistryCredentialID: runtimeCfg.RegistryCredentialID, ImageRef: runtimeCfg.ImageRef, ImageDigest: runtimeCfg.ImageDigest, InternalPort: runtimeCfg.InternalPort, Protocol: runtimeCfg.Protocol, FlagFormat: runtimeCfg.FlagFormat, CPUMilli: runtimeCfg.CPUMilli, MemoryMB: runtimeCfg.MemoryMB, PIDsLimit: runtimeCfg.PIDsLimit, DiskMB: runtimeCfg.DiskMB, TTLSeconds: runtimeCfg.TTLSeconds, MaxTTLSeconds: runtimeCfg.MaxTTLSeconds, ReadOnlyRootFS: runtimeCfg.ReadOnlyRootFS, EnvironmentTemplate: runtimeCfg.EnvironmentTemplate, HealthcheckJSON: json.RawMessage(`{}`), CreatedAt: now}
			if err := tx.Create(&runtimeRevision).Error; err != nil {
				return err
			}
		}
		if !challenge.IsDynamic {
			var flagCount int64
			if err := tx.Model(&models.ChallengeFlag{}).Where("challenge_id=? AND enabled=true", challenge.ID).Count(&flagCount).Error; err != nil {
				return err
			}
			if flagCount == 0 {
				return httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_FLAG_REQUIRED", "静态题目发布前必须配置至少一个 Flag")
			}
		}
		if err := tx.Exec(`INSERT INTO challenge_flag_revisions(id,challenge_revision_id,source_flag_id,kind,hmac,regex_pattern,stage,created_at)
SELECT gen_random_uuid(),?,id,kind,hmac,regex_pattern,stage,? FROM challenge_flags WHERE challenge_id=? AND enabled=true`, revision.ID, now, challenge.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO challenge_file_revisions(id,challenge_revision_id,source_file_id,name,object_key,mime_type,sha256,size,public,created_at)
SELECT gen_random_uuid(),?,id,name,object_key,mime_type,sha256,size,public,? FROM challenge_files WHERE challenge_id=? AND archived_at IS NULL`, revision.ID, now, challenge.ID).Error; err != nil {
			return err
		}
		if err := tx.Model(&challenge).Updates(map[string]any{"current_published_revision_id": revision.ID, "status": "published", "published_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := learningmodule.SyncPublishedChallenge(tx, challenge, now); err != nil {
			return err
		}
		return tx.Table("challenge_publication_logs").Create(map[string]any{"id": uuid.New(), "challenge_id": challenge.ID, "actor_id": actor, "action": "publish", "version": version, "created_at": now}).Error
	})
}

func (s *Service) Archive(ctx context.Context, id string, actor uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var challenge models.Challenge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id::text=? OR slug=?", id, id).First(&challenge).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
			}
			return err
		}
		if challenge.Status == "archived" {
			return nil
		}
		var activeCompetitionCount int64
		if err := tx.Raw(`SELECT count(*) FROM competitions c
WHERE c.status IN ('draft','registration','running','frozen')
  AND (
    EXISTS (SELECT 1 FROM competition_challenges cc WHERE cc.competition_id=c.id AND cc.challenge_id=?)
    OR EXISTS (
      SELECT 1 FROM competition_challenge_snapshots ccs
      WHERE ccs.competition_snapshot_id=c.current_snapshot_id AND ccs.challenge_id=?
    )
  )`, challenge.ID, challenge.ID).Scan(&activeCompetitionCount).Error; err != nil {
			return err
		}
		if activeCompetitionCount > 0 {
			return httpx.NewError(http.StatusConflict, "CHALLENGE_IN_ACTIVE_COMPETITION", "题目正在待发布、报名中或进行中的比赛内使用，暂不能归档")
		}
		now := time.Now().UTC()
		if err := tx.Model(&challenge).Updates(map[string]any{"status": "archived", "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Table("challenge_runtime_configs").Where("challenge_id=?", challenge.ID).Update("enabled", false).Error; err != nil {
			return err
		}
		if err := learningmodule.RemoveManagedChallenge(tx, challenge.ID, now); err != nil {
			return err
		}
		var version int
		if err := tx.Table("challenge_revisions").Where("challenge_id=?", challenge.ID).Select("COALESCE(MAX(version),0)").Scan(&version).Error; err != nil {
			return err
		}
		return tx.Table("challenge_publication_logs").Create(map[string]any{"id": uuid.New(), "challenge_id": challenge.ID, "actor_id": actor, "action": "archive", "version": version, "created_at": now}).Error
	})
}

func imageRegistryHost(imageRef string) string {
	first, _, hasPath := strings.Cut(strings.TrimSpace(imageRef), "/")
	if !hasPath {
		return "docker.io"
	}
	first = strings.ToLower(first)
	if first == "localhost" || strings.Contains(first, ".") || strings.Contains(first, ":") {
		return first
	}
	return "docker.io"
}

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var environmentKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
var imageRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?::[0-9]+)?(?:/[A-Za-z0-9][A-Za-z0-9._-]*)*(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?(?:@sha256:[a-f0-9]{64})?$`)

func normalizeMutation(input *Mutation) error {
	input.Title = strings.TrimSpace(input.Title)
	input.CategoryKey = strings.TrimSpace(input.CategoryKey)
	input.Author = strings.TrimSpace(input.Author)
	if input.Title == "" || input.CategoryKey == "" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_CHALLENGE", "题目标题和方向不能为空")
	}
	if input.Difficulty == "" {
		input.Difficulty = "入门"
	}
	if input.ScoreMode == "" {
		input.ScoreMode = "fixed"
	}
	if input.ScoreMode != "fixed" && input.ScoreMode != "dynamic" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_SCORE_MODE", "计分模式仅支持 fixed 或 dynamic")
	}
	if input.Visibility == "" {
		input.Visibility = "public"
	}
	if input.Visibility != "public" && input.Visibility != "private" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_VISIBILITY", "可见性仅支持 public 或 private")
	}
	if input.BaseScore <= 0 {
		input.BaseScore = 100
	}
	if input.MinimumScore <= 0 {
		input.MinimumScore = input.BaseScore
	}
	if input.MaximumScore <= 0 {
		input.MaximumScore = input.BaseScore
	}
	if input.MinimumScore > input.BaseScore || input.BaseScore > input.MaximumScore {
		return httpx.NewError(http.StatusBadRequest, "INVALID_SCORE_RANGE", "题目分值必须满足最低分 ≤ 基础分 ≤ 最高分")
	}
	if input.DynamicDecay <= 0 {
		input.DynamicDecay = 50
	}
	return nil
}

func validateRuntime(runtime *RuntimeMutation) error {
	if runtime == nil {
		return httpx.NewError(http.StatusBadRequest, "RUNTIME_NOT_CONFIGURED", "动态题目必须配置运行环境")
	}
	runtime.ImageRef = strings.TrimSpace(runtime.ImageRef)
	runtime.ImageDigest = strings.ToLower(strings.TrimSpace(runtime.ImageDigest))
	runtime.Protocol = strings.ToLower(strings.TrimSpace(runtime.Protocol))
	runtime.FlagFormat = strings.ToLower(strings.TrimSpace(runtime.FlagFormat))
	if runtime.Protocol == "" {
		runtime.Protocol = "tcp"
	}
	if runtime.FlagFormat == "" {
		runtime.FlagFormat = "standard"
	}
	if len(runtime.ImageRef) > 512 || !imageRefPattern.MatchString(runtime.ImageRef) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_IMAGE", "容器镜像名称不合法，请填写本机已有的 image:tag 或固定的 image@sha256 引用")
	}
	if runtime.ImageDigest == "" {
		if strings.Contains(runtime.ImageRef, "@") {
			return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_IMAGE", "固定镜像引用必须包含有效的 sha256 Digest")
		}
	} else if !digestPattern.MatchString(runtime.ImageDigest) || !strings.HasSuffix(runtime.ImageRef, "@"+runtime.ImageDigest) {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_IMAGE", "镜像引用与 Digest 不一致")
	}
	if runtime.InternalPort < 1 || runtime.InternalPort > 65535 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_PORT", "容器内部端口必须在 1 到 65535 之间")
	}
	if runtime.Protocol != "http" && runtime.Protocol != "https" && runtime.Protocol != "tcp" && runtime.Protocol != "udp" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_PROTOCOL", "协议仅支持 http、https、tcp 或 udp")
	}
	if runtime.FlagFormat != "standard" && runtime.FlagFormat != "uuid" {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_FLAG_FORMAT", "动态 Flag 格式仅支持 standard 或 uuid")
	}
	if runtime.CPUMilli < 50 || runtime.CPUMilli > 8000 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_CPU", "CPU 配额必须在 50 到 8000m 之间")
	}
	if runtime.MemoryMB < 32 || runtime.MemoryMB > 16384 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_MEMORY", "内存必须在 32 到 16384 MB 之间")
	}
	if runtime.PIDsLimit < 16 || runtime.PIDsLimit > 4096 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_PIDS", "进程数上限必须在 16 到 4096 之间")
	}
	if runtime.DiskMB < 16 || runtime.DiskMB > 102400 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_DISK", "磁盘预算必须在 16 到 102400 MB 之间")
	}
	if runtime.TTLSeconds < 300 || runtime.TTLSeconds > 86400 || runtime.MaxTTLSeconds < runtime.TTLSeconds || runtime.MaxTTLSeconds > 604800 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_TTL", "默认时长需为 5 分钟到 24 小时，最大时长需不小于默认值且不超过 7 天")
	}
	if len(runtime.Environment) > 32 {
		return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_ENV", "环境变量最多 32 个")
	}
	for key, value := range runtime.Environment {
		if key == "ASAMU_FLAG" || key == "ASAMU_INSTANCE_ID" || key == "CM_FLAG" || key == "CM_INSTANCE_ID" || !environmentKeyPattern.MatchString(key) || len(value) > 2048 {
			return httpx.NewError(http.StatusBadRequest, "INVALID_RUNTIME_ENV", "环境变量名称或长度不合法，且不能覆盖平台保留变量")
		}
	}
	return nil
}
