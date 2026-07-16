package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/modules/challenge"
	"asamu.local/platform/api/internal/modules/competition"
	"asamu.local/platform/api/internal/modules/scoreboard"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Seeder struct {
	db         *gorm.DB
	flagSecret []byte
}

func New(db *gorm.DB, flagSecret string) *Seeder {
	return &Seeder{db: db, flagSecret: []byte(flagSecret)}
}

func (s *Seeder) Run(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize and atomically commit the complete seed. This protects every
		// remaining check-then-create path even if two init jobs overlap.
		const seedAdvisoryLockKey int64 = 0x4153414d55534545
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", seedAdvisoryLockKey).Error; err != nil {
			return fmt.Errorf("acquire seed lock: %w", err)
		}
		scoped := &Seeder{db: tx, flagSecret: s.flagSecret}
		return scoped.runLocked(ctx)
	})
}

func (s *Seeder) runLocked(ctx context.Context) error {
	if err := s.roles(ctx); err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}
	if err := s.categories(ctx); err != nil {
		return fmt.Errorf("seed challenge categories: %w", err)
	}
	if err := s.platform(ctx); err != nil {
		return fmt.Errorf("seed platform profile: %w", err)
	}
	if err := s.levels(ctx); err != nil {
		return fmt.Errorf("seed progression levels: %w", err)
	}
	if err := s.assets(ctx); err != nil {
		return fmt.Errorf("seed assets: %w", err)
	}
	adminID, err := s.userFromEnv(ctx, "DEV_ADMIN_", []string{"super_admin", "user"})
	if err != nil {
		return fmt.Errorf("seed administrator: %w", err)
	}
	userID, err := s.userFromEnv(ctx, "DEV_USER_", []string{"user"})
	if err != nil {
		return fmt.Errorf("seed development user: %w", err)
	}
	author := adminID
	if author == uuid.Nil {
		author = userID
	}
	if author != uuid.Nil && envBool("SEED_DEMO_CONTENT", false) {
		demoManaged, err := s.content(ctx, author)
		if err != nil {
			return fmt.Errorf("seed demo content: %w", err)
		}
		if demoManaged {
			var challengeState struct {
				CurrentPublishedRevisionID *uuid.UUID
				Status                     string
				AuthorName                 string
			}
			if err := s.db.WithContext(ctx).Table("challenges").Select("current_published_revision_id,status,author_name").Where("slug='sqli-art'").Take(&challengeState).Error; err != nil {
				return err
			}
			if challengeState.CurrentPublishedRevisionID == nil && challengeState.Status == "draft" && challengeState.AuthorName == "asamu Lab" {
				if err := challenge.New(s.db, string(s.flagSecret)).Publish(ctx, "sqli-art", author); err != nil {
					return err
				}
			}
			if err := s.makePracticeCompetitionPlayable(ctx); err != nil {
				return err
			}
			var competitionState struct {
				CurrentSnapshotID *uuid.UUID
				Status            string
				Name              string
			}
			if err := s.db.WithContext(ctx).Table("competitions").Select("current_snapshot_id,status,name").Where("slug='asamu-practice'").Take(&competitionState).Error; err != nil {
				return err
			}
			if competitionState.CurrentSnapshotID == nil && competitionState.Status == "registration" && competitionState.Name == "asamu 新生练习赛" {
				if err := competition.New(s.db, scoreboard.New(s.db)).SetStatus(ctx, "asamu-practice", "running", author); err != nil {
					return err
				}
			}
		}
	}
	if err := s.learning(ctx); err != nil {
		return fmt.Errorf("seed learning center: %w", err)
	}
	return nil
}
func (s *Seeder) platform(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		profileID := uuid.New()
		profile := map[string]any{"id": profileID, "profile_key": "default", "platform_name": "asamu", "short_name": "ASAMU", "slogan": "探索无界 · 攻防未来", "description": "网络安全学习与竞赛平台", "contact_json": json.RawMessage(`{}`), "default_locale": "zh-CN", "timezone": "Asia/Shanghai", "homepage_title": "asamu 网络安全学习平台", "default_theme_key": "platform-default", "runtime_defaults_json": json.RawMessage(`{}`), "status": "draft", "version": 1, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}
		if err := tx.Table("platform_profiles").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "profile_key"}}, DoNothing: true}).Create(profile).Error; err != nil {
			return err
		}
		var profileState struct{ ID uuid.UUID }
		if err := tx.Table("platform_profiles").Select("id").Where("profile_key='default'").Take(&profileState).Error; err != nil {
			return err
		}
		profileID = profileState.ID
		features := map[string]bool{"registration": true, "teams": true, "writeups": true, "learning": true, "competitions": true, "runtime": false}
		for key, enabled := range features {
			if err := tx.Table("platform_feature_flags").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"profile_id": profileID, "feature_key": key, "enabled": enabled, "config_json": json.RawMessage(`{}`), "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		nav := []struct{ Key, Label, Href string }{{"home", "首页", "/"}, {"challenges", "题库", "/challenges"}, {"competitions", "比赛", "/competitions"}, {"teams", "战队", "/teams"}, {"leaderboard", "排名", "/leaderboard"}, {"writeups", "WriteUp", "/writeups"}, {"learning", "学习中心", "/learning"}}
		for index, item := range nav {
			if err := tx.Table("navigation_items").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"id": uuid.New(), "profile_id": profileID, "item_key": item.Key, "label": item.Label, "href": item.Href, "sort_order": index, "enabled": true}).Error; err != nil {
				return err
			}
		}
		return tx.Table("challenge_library_configs").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"id": uuid.New(), "profile_id": profileID, "page_title": "探索题库", "page_subtitle": "选择方向，完成真实挑战", "search_placeholder": "搜索题目", "show_direction_section": true, "show_sidebar": true, "filter_groups_json": json.RawMessage(`[]`), "default_sort": "direction", "page_size": 20, "card_fields_json": json.RawMessage(`["difficulty","score","solves","tags"]`), "empty_state_json": json.RawMessage(`{}`), "error_state_json": json.RawMessage(`{}`), "updated_at": time.Now().UTC()}).Error
	})
}
func (s *Seeder) roles(ctx context.Context) error {
	permissions := []string{"user.read", "user.ban", "rbac.manage", "challenge.read", "challenge.write", "challenge.publish", "competition.read", "competition.write", "competition.publish", "instance.read", "instance.manage", "registry.read", "registry.manage", "submission.read", "anticheat.read", "anticheat.review", "writeup.review", "announcement.write", "audit.read", "asset.read", "asset.upload", "asset.publish", "asset.rollback", "asset.archive", "asset.manage_taxonomy", "appearance.read", "appearance.write", "appearance.publish", "appearance.rollback", "progression.manage", "reward.manage", "platform.read", "platform.write", "platform.publish", "direction.read", "direction.write", "direction.archive", "scoring.adjust", "scoring.rebuild"}
	roles := map[string][]string{"super_admin": {"*"}, "site_admin": permissions, "visual_operator": {"asset.read", "asset.upload", "asset.publish", "asset.rollback", "asset.archive", "asset.manage_taxonomy", "appearance.read", "appearance.write", "appearance.publish", "appearance.rollback"}, "competition_admin": {"competition.read", "competition.write", "competition.publish", "instance.read", "instance.manage", "submission.read", "anticheat.read", "anticheat.review"}, "challenge_author": {"challenge.read", "challenge.write", "instance.read", "registry.read", "submission.read"}, "reviewer": {"writeup.review", "anticheat.read", "anticheat.review"}, "team_captain": {}, "user": {}}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permissionIDs := map[string]uuid.UUID{}
		for _, key := range permissions {
			candidate := models.Permission{ID: uuid.New(), Key: key, Name: key}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return err
			}
			var permission models.Permission
			if err := tx.Where("key=?", key).Take(&permission).Error; err != nil {
				return err
			}
			permissionIDs[key] = permission.ID
		}
		starCandidate := models.Permission{ID: uuid.New(), Key: "*", Name: "全部权限"}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&starCandidate).Error; err != nil {
			return err
		}
		var star models.Permission
		if err := tx.Where("key='*'").Take(&star).Error; err != nil {
			return err
		}
		permissionIDs["*"] = star.ID
		for key, list := range roles {
			candidate := models.Role{ID: uuid.New(), Key: key, Name: key}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return err
			}
			var role models.Role
			if err := tx.Where("key=?", key).Take(&role).Error; err != nil {
				return err
			}
			for _, permission := range list {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.RolePermission{RoleID: role.ID, PermissionID: permissionIDs[permission]}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
func (s *Seeder) categories(ctx context.Context) error {
	values := []struct{ Key, Name, Asset string }{{"web", "Web", "direction.web.scene"}, {"pwn", "Pwn", "direction.pwn.scene"}, {"reverse", "Reverse", "direction.reverse.scene"}, {"crypto", "Crypto", "direction.crypto.scene"}, {"misc", "Misc", "direction.misc.scene"}, {"forensics", "Forensics", "direction.forensics.scene"}, {"iot", "IoT", "direction.iot.scene"}, {"mobile", "Mobile", "direction.mobile.scene"}, {"cloud", "Cloud", "direction.cloud.scene"}, {"ai-security", "AI Security", "direction.ai_security.scene"}, {"blockchain", "Blockchain", "direction.blockchain.scene"}, {"osint", "OSINT", "direction.osint.scene"}, {"ics", "ICS", "direction.ics.scene"}}
	activeDirections := map[string]bool{"web": true, "misc": true, "reverse": true, "mobile": true, "pwn": true, "iot": true, "crypto": true}
	for index, value := range values {
		active := activeDirections[value.Key]
		candidate := models.ChallengeCategory{ID: uuid.New(), Key: value.Key, Name: value.Name, SceneAssetKey: value.Asset, SortOrder: index, Enabled: active}
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
		// Always query into a fresh zero-value model. Reusing candidate makes GORM
		// append candidate.ID to the WHERE clause after an ON CONFLICT update.
		var record models.ChallengeCategory
		if err := s.db.WithContext(ctx).Where("key=?", value.Key).Take(&record).Error; err != nil {
			return err
		}
		status := "archived"
		if active {
			status = "active"
		}
		direction := map[string]any{"id": record.ID, "slug": value.Key, "name": value.Name, "card_asset_key": value.Asset, "sort_order": index, "status": status, "show_on_home": active, "show_on_library_header": active, "show_on_library_sidebar": active, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}
		if err := s.db.WithContext(ctx).Table("challenge_directions").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(direction).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Seeder) levels(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		schemeID := uuid.New()
		if err := tx.Table("level_schemes").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(map[string]any{"id": schemeID, "key": "platform-default", "name": "平台长期等级", "scope_type": "platform", "enabled": true, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		var schemeState struct{ ID uuid.UUID }
		if err := tx.Table("level_schemes").Select("id").Where("key='platform-default'").Take(&schemeState).Error; err != nil {
			return err
		}
		schemeID = schemeState.ID
		tiers := []struct {
			Key, Name, Asset string
			Min              int64
		}{{"bronze", "青铜", "rank.bronze.main", 0}, {"silver", "白银", "rank.silver.main", 500}, {"gold", "黄金", "rank.gold.main", 1500}, {"platinum", "铂金", "rank.platinum.main", 3500}, {"diamond", "钻石", "rank.diamond.main", 7000}, {"master", "大师", "rank.master.main", 12000}, {"king", "王者", "rank.king.main", 20000}, {"legend", "超神", "rank.legend.main", 35000}}
		for index, tier := range tiers {
			record := map[string]any{"id": uuid.New(), "scheme_id": schemeID, "key": tier.Key, "name": tier.Name, "min_experience": tier.Min, "sort_order": index, "badge_asset_key": tier.Asset, "color": "#1677ff"}
			if err := tx.Table("level_tiers").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scheme_id"}, {Name: "key"}}, DoNothing: true}).Create(record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Seeder) learning(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var categories []models.ChallengeCategory
		if err := tx.Where("enabled=true").Order("sort_order,name").Find(&categories).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, category := range categories {
			var challengeIDs []uuid.UUID
			if err := tx.Table("challenges").Where("category_id=? AND status='published' AND visibility='public'", category.ID).Order("base_score,title").Pluck("id", &challengeIDs).Error; err != nil {
				return err
			}
			slug := category.Key + "-foundation"
			pathID := uuid.New()
			status := "draft"
			var publishedAt *time.Time
			if len(challengeIDs) > 0 {
				status = "published"
				publishedAt = &now
			}
			if err := tx.Table("learning_paths").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(map[string]any{"id": pathID, "slug": slug, "direction_id": category.ID, "title": category.Name + " 安全训练路线", "summary": "从基础知识到综合实战，循序完成 " + category.Name + " 方向训练。", "description": "按照阶段顺序完成已发布题目，解题进度会自动同步到学习中心。", "prerequisite": "Linux 基础 / 网络基础", "estimated_minutes": 720, "hero_asset_key": category.SceneAssetKey, "status": status, "featured": category.Key == "web", "sort_order": category.SortOrder, "published_at": publishedAt, "created_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			var path struct {
				ID        uuid.UUID
				Status    string
				CreatedBy *uuid.UUID
			}
			if err := tx.Table("learning_paths").Select("id,status,created_by").Where("slug=?", slug).Take(&path).Error; err != nil {
				return err
			}
			if path.Status == "archived" {
				continue
			}
			if path.CreatedBy == nil && path.Status != "archived" {
				updates := map[string]any{"updated_at": now}
				if len(challengeIDs) == 0 {
					updates["status"] = "draft"
					updates["featured"] = false
					updates["published_at"] = nil
				} else if path.Status == "draft" {
					updates["status"] = "published"
					updates["published_at"] = now
				}
				if err := tx.Table("learning_paths").Where("id=?", path.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			var stageIDs []uuid.UUID
			if err := tx.Table("learning_stages").Where("path_id=?", path.ID).Order("sort_order,title").Pluck("id", &stageIDs).Error; err != nil {
				return err
			}
			if len(stageIDs) == 0 {
				stageTitles := []string{"基础入门", "核心技能", "综合实战"}
				stageIDs = make([]uuid.UUID, len(stageTitles))
				for index, title := range stageTitles {
					stageIDs[index] = uuid.New()
					if err := tx.Table("learning_stages").Create(map[string]any{"id": stageIDs[index], "path_id": path.ID, "title": title, "description": "完成本阶段编排的 " + category.Name + " 题目。", "sort_order": index + 1, "created_at": now, "updated_at": now}).Error; err != nil {
						return err
					}
				}
			}
			if path.CreatedBy != nil || len(stageIDs) == 0 {
				continue
			}
			var assignedIDs []uuid.UUID
			if err := tx.Table("learning_stage_challenges lsc").Joins("JOIN learning_stages ls ON ls.id=lsc.stage_id").Where("ls.path_id=?", path.ID).Pluck("lsc.challenge_id", &assignedIDs).Error; err != nil {
				return err
			}
			assigned := make(map[uuid.UUID]bool, len(assignedIDs))
			for _, challengeID := range assignedIDs {
				assigned[challengeID] = true
			}
			for index, challengeID := range challengeIDs {
				if assigned[challengeID] {
					continue
				}
				stageIndex := index * len(stageIDs) / max(1, len(challengeIDs))
				if stageIndex >= len(stageIDs) {
					stageIndex = len(stageIDs) - 1
				}
				if err := tx.Table("learning_stage_challenges").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"stage_id": stageIDs[stageIndex], "challenge_id": challengeID, "sort_order": index + 1, "required": true}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
func (s *Seeder) userFromEnv(ctx context.Context, prefix string, roles []string) (uuid.UUID, error) {
	email, username, password := os.Getenv(prefix+"EMAIL"), os.Getenv(prefix+"USERNAME"), os.Getenv(prefix+"PASSWORD")
	if email == "" && username == "" && password == "" {
		return uuid.Nil, nil
	}
	if email == "" || username == "" || password == "" {
		return uuid.Nil, fmt.Errorf("%sEMAIL, USERNAME and PASSWORD must all be configured", prefix)
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	var user models.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("email=?", email).Limit(1).Find(&user)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			user = models.User{ID: uuid.New(), Email: email, Username: username, PasswordHash: hash, Status: "active"}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.UserProfile{UserID: user.ID, DisplayName: username, Skills: []byte("[]"), Privacy: []byte("{}")}).Error; err != nil {
				return err
			}
		}
		for _, roleKey := range roles {
			var role models.Role
			if err := tx.Where("key=?", roleKey).First(&role).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return user.ID, err
}
func (s *Seeder) content(ctx context.Context, author uuid.UUID) (bool, error) {
	managed := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var category models.ChallengeCategory
		if err := tx.Where("key='web'").First(&category).Error; err != nil {
			return err
		}
		candidate := models.Challenge{ID: uuid.New(), Slug: "sqli-art", CategoryID: category.ID, DirectionID: &category.ID, Title: "SQLi 是门艺术", Difficulty: "中等", Summary: "从检索参数中发现隐藏的 SQL 查询路径。", DescriptionMarkdown: "欢迎来到在线书店。观察搜索参数，找到管理员收藏的秘密记录。", AuthorName: "asamu Lab", Status: "draft", Visibility: "public", ScoreMode: "dynamic", BaseScore: 300, MinimumScore: 100, MaximumScore: 500, DynamicDecay: 50, IsDynamic: true}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
		var challenge models.Challenge
		if err := tx.Where("slug='sqli-art'").Take(&challenge).Error; err != nil {
			return err
		}
		if challenge.AuthorName != "asamu Lab" {
			return nil
		}
		runtimeImage := strings.TrimSpace(os.Getenv("DEFAULT_CHALLENGE_IMAGE"))
		if runtimeImage == "" {
			runtimeImage = "asamu/sqli-lab:dev"
		}
		internalPort, parseErr := strconv.Atoi(os.Getenv("DEFAULT_CHALLENGE_INTERNAL_PORT"))
		if parseErr != nil || internalPort < 1 || internalPort > 65535 {
			internalPort = 8080
		}
		imageDigest := ""
		if separator := strings.Index(runtimeImage, "@sha256:"); separator >= 0 {
			imageDigest = runtimeImage[separator+1:]
		}
		runtime := models.ChallengeRuntimeConfig{ID: uuid.New(), ChallengeID: challenge.ID, ImageRef: runtimeImage, ImageDigest: imageDigest, InternalPort: internalPort, Protocol: "http", CPUMilli: 250, MemoryMB: 128, PIDsLimit: 64, DiskMB: 64, TTLSeconds: 7200, MaxTTLSeconds: 14400, ReadOnlyRootFS: true, EnvironmentTemplate: []byte("{}"), Enabled: true}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "challenge_id"}}, DoNothing: true}).Create(&runtime).Error; err != nil {
			return err
		}
		var hintCount int64
		if err := tx.Model(&models.ChallengeHint{}).Where("challenge_id=?", challenge.ID).Count(&hintCount).Error; err != nil {
			return err
		}
		if hintCount == 0 {
			for index, hint := range []string{"比较正常搜索和带引号搜索的响应差异。", "尝试 UNION SELECT 并关注列数。"} {
				record := models.ChallengeHint{ID: uuid.New(), ChallengeID: challenge.ID, Title: fmt.Sprintf("Hint %d", index+1), ContentMarkdown: hint, SortOrder: index}
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
			}
		}
		now := time.Now().UTC()
		competitionCandidate := models.Competition{ID: uuid.New(), Slug: "asamu-practice", Name: "asamu 新生练习赛", Summary: "可重复参加的动态靶场练习赛", Mode: "individual", Status: "registration", ScoringMode: "dynamic", Visibility: "public", BannerAssetKey: "competition.hero", RegistrationStartsAt: now.Add(-7 * 24 * time.Hour), RegistrationEndsAt: now.Add(-48 * time.Hour), StartsAt: now.Add(-24 * time.Hour), EndsAt: now.Add(30 * 24 * time.Hour), TeamMin: 1, TeamMax: 1}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(&competitionCandidate).Error; err != nil {
			return err
		}
		var competitionRecord models.Competition
		if err := tx.Where("slug='asamu-practice'").Take(&competitionRecord).Error; err != nil {
			return err
		}
		managed = competitionRecord.Name == "asamu 新生练习赛"
		return nil
	})
	return managed, err
}
func (s *Seeder) assets(ctx context.Context) error {
	for _, category := range []struct{ Key, Name string }{{"home", "首页"}, {"direction", "方向"}, {"team", "战队"}, {"rank", "等级"}, {"honor", "荣誉"}, {"environment", "环境"}} {
		if err := s.db.WithContext(ctx).Table("asset_categories").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"id": uuid.New(), "key": category.Key, "name": category.Name, "created_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
	}
	manifestPath := os.Getenv("ASSET_MANIFEST_PATH")
	if manifestPath == "" {
		manifestPath = filepath.Clean("../web/public/assets/default/manifest.v3.json")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read default asset manifest %s: %w", manifestPath, err)
	}
	if err == nil {
		var manifest struct {
			Release string `json:"release"`
			Assets  map[string]struct {
				Path                                            string
				Width, Height                                   int
				HasAlpha                                        bool
				SHA256                                          string
				SafeArea, FocalPoint                            map[string]int
				RecommendedObjectFit, RecommendedObjectPosition string
			}
		}
		if err := json.Unmarshal(data, &manifest); err == nil {
			keys := make([]string, 0, len(manifest.Assets))
			for key := range manifest.Assets {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				value := manifest.Assets[key]
				var existing int64
				if err := s.db.WithContext(ctx).Table("assets").Where("asset_key=?", key).Count(&existing).Error; err != nil {
					return err
				}
				if existing > 0 {
					continue
				}
				assetID, versionID := uuid.New(), uuid.New()
				categoryKey := strings.Split(key, ".")[0]
				var categoryID *uuid.UUID
				var categoryState struct{ ID uuid.UUID }
				categoryLookup := s.db.WithContext(ctx).Table("asset_categories").Select("id").Where("key=?", categoryKey).Limit(1).Find(&categoryState)
				if categoryLookup.Error == nil && categoryLookup.RowsAffected > 0 && categoryState.ID != uuid.Nil {
					categoryID = &categoryState.ID
				}
				safe, _ := json.Marshal(value.SafeArea)
				focal, _ := json.Marshal(value.FocalPoint)
				pages, _ := json.Marshal([]string{"global"})
				if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					if err := tx.Table("assets").Create(map[string]any{"id": assetID, "asset_key": key, "name": key, "category_id": categoryID, "alt_text": key, "status": "published", "fit": value.RecommendedObjectFit, "position": value.RecommendedObjectPosition, "focal_point": focal, "safe_area": safe, "applicable_pages": pages, "current_version": 1, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error; err != nil {
						return err
					}
					return tx.Table("asset_versions").Create(map[string]any{"id": versionID, "asset_id": assetID, "version": 1, "object_key": value.Path, "public_url": value.Path, "mime_type": defaultAssetMIME(value.Path), "width": value.Width, "height": value.Height, "has_alpha": value.HasAlpha, "sha256": value.SHA256, "risk_flags": []byte("[]"), "note": manifest.Release, "created_at": time.Now().UTC()}).Error
				}); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("parse default asset manifest %s: %w", manifestPath, err)
		}
	}
	slots := []struct{ Key, Name, Page string }{{"home.hero", "首页 Hero", "home"}, {"competition.hero", "比赛主视觉", "competitions"}, {"team.base.hero", "战队基地", "teams"}, {"training.route.hero", "训练路线", "learning"}}
	for _, slot := range slots {
		slotID := uuid.New()
		if err := s.db.WithContext(ctx).Table("asset_slots").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slot_key"}}, DoNothing: true}).Create(map[string]any{"id": slotID, "slot_key": slot.Key, "name": slot.Name, "page_key": slot.Page, "fit": "contain", "position": "center", "enabled": true, "version": 1, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		var slotRow struct{ ID uuid.UUID }
		if err := s.db.WithContext(ctx).Table("asset_slots").Select("id").Where("slot_key=?", slot.Key).Take(&slotRow).Error; err != nil {
			return err
		}
		var versionRow struct{ ID uuid.UUID }
		lookup := s.db.WithContext(ctx).Table("asset_versions v").Select("v.id").Joins("JOIN assets a ON a.id=v.asset_id AND a.current_version=v.version").Where("a.asset_key=?", slot.Key).Take(&versionRow)
		if lookup.Error == nil {
			var bindingCount int64
			if err := s.db.WithContext(ctx).Table("asset_bindings").Where("slot_id=? AND status='published'", slotRow.ID).Count(&bindingCount).Error; err != nil {
				return err
			}
			if bindingCount == 0 {
				if err := s.db.WithContext(ctx).Table("asset_bindings").Create(map[string]any{"id": uuid.New(), "slot_id": slotRow.ID, "scope_type": "platform", "light_desktop_version_id": versionRow.ID, "priority": 0, "status": "published", "created_at": time.Now().UTC()}).Error; err != nil {
					return err
				}
			}
		} else if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
	}
	for _, page := range []string{"global", "home", "challenges", "challenge_detail", "learning", "competitions", "team_list", "team_detail", "leaderboard", "writeups", "profile", "login", "admin"} {
		var count int64
		if err := s.db.WithContext(ctx).Table("page_background_configs").Where("page_key=?", page).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			overlayOpacity, assetOpacity := 55, 12
			if page == "home" {
				overlayOpacity, assetOpacity = 45, 18
			}
			if err := s.db.WithContext(ctx).Table("page_background_configs").Create(map[string]any{"id": uuid.New(), "page_key": page, "scope_type": "platform", "light_asset_key": "background.platform.light", "fit": "cover", "position": "center", "focal_point": []byte(`{"x":50,"y":50}`), "overlay_color": "#ffffff", "overlay_opacity": overlayOpacity, "asset_opacity": assetOpacity, "blur": 0, "status": "published", "version": 1, "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func defaultAssetMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	default:
		return "image/webp"
	}
}

func timePointer(value time.Time) *time.Time { return &value }

func (s *Seeder) makePracticeCompetitionPlayable(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var competition models.Competition
		if err := tx.Where("slug=?", "asamu-practice").First(&competition).Error; err != nil {
			return err
		}
		if competition.Name != "asamu 新生练习赛" {
			return nil
		}
		var challenge models.Challenge
		if err := tx.Where("slug=?", "sqli-art").First(&challenge).Error; err != nil {
			return err
		}
		if challenge.AuthorName != "asamu Lab" {
			return nil
		}
		now := time.Now().UTC()
		relation := models.CompetitionChallenge{
			CompetitionID: competition.ID,
			ChallengeID:   challenge.ID,
			Score:         challenge.BaseScore,
			SortOrder:     1,
			OpensAt:       timePointer(now.Add(-24 * time.Hour)),
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "competition_id"}, {Name: "challenge_id"}},
			DoNothing: true,
		}).Create(&relation).Error
	})
}

func sha(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

var _ = sha
