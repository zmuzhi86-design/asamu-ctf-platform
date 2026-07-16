package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/modules/admin"
	"asamu.local/platform/api/internal/modules/appearance"
	"asamu.local/platform/api/internal/modules/asset"
	"asamu.local/platform/api/internal/modules/auth"
	"asamu.local/platform/api/internal/modules/challenge"
	"asamu.local/platform/api/internal/modules/challengefile"
	"asamu.local/platform/api/internal/modules/competition"
	"asamu.local/platform/api/internal/modules/hint"
	"asamu.local/platform/api/internal/modules/instance"
	"asamu.local/platform/api/internal/modules/learning"
	"asamu.local/platform/api/internal/modules/notification"
	"asamu.local/platform/api/internal/modules/organization"
	"asamu.local/platform/api/internal/modules/platformconfig"
	"asamu.local/platform/api/internal/modules/progression"
	"asamu.local/platform/api/internal/modules/registrycredential"
	"asamu.local/platform/api/internal/modules/scoreboard"
	"asamu.local/platform/api/internal/modules/submission"
	"asamu.local/platform/api/internal/modules/team"
	"asamu.local/platform/api/internal/modules/user"
	"asamu.local/platform/api/internal/modules/writeup"
	"asamu.local/platform/api/internal/platform/cache"
	"asamu.local/platform/api/internal/platform/database"
	"asamu.local/platform/api/internal/platform/httpx"
	maildispatcher "asamu.local/platform/api/internal/platform/mail"
	"asamu.local/platform/api/internal/platform/observability"
	"asamu.local/platform/api/internal/platform/queue"
	"asamu.local/platform/api/internal/platform/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type App struct {
	Config   config.Config
	Logger   *zap.Logger
	Database *database.Database
	Cache    *cache.Redis
	Storage  storage.Storage
	Stream   *queue.Stream
	Router   *gin.Engine
	Mailer   *maildispatcher.Dispatcher
}

func Build(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger, err := observability.NewLogger(cfg.Environment)
	if err != nil {
		return nil, err
	}
	db, err := database.Open(cfg.Database, cfg.Environment)
	if err != nil {
		return nil, err
	}
	redisClient, err := cache.Open(cfg.Redis)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store, err := storage.Open(cfg.Storage)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		return nil, err
	}
	stream := queue.NewStream(redisClient.Client, cfg.Redis.Stream, cfg.Redis.ConsumerGroup, "api")
	if err := stream.Ensure(ctx); err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		return nil, err
	}
	app := &App{Config: cfg, Logger: logger, Database: db, Cache: redisClient, Storage: store, Stream: stream}
	app.Router = routes(app)
	app.Mailer = maildispatcher.NewDispatcher(db.GORM, cfg.Mail, cfg.Security.ConfirmationTokenSecret, logger)
	app.Mailer.Start(ctx)
	return app, nil
}
func (a *App) Close() {
	if a.Mailer != nil {
		a.Mailer.Close()
	}
	_ = a.Cache.Close()
	_ = a.Database.Close()
	_ = a.Logger.Sync()
}
func routes(app *App) *gin.Engine {
	if app.Config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(observability.RequestLogger(app.Logger), httpx.Recovery(app.Logger), httpx.SecurityHeaders(), httpx.CORS(app.Config.WebOrigins))
	rateLimiter := httpx.NewRateLimiter(app.Cache.Client, app.Config.RateLimit.RPS, app.Config.RateLimit.Burst)
	router.Use(rateLimiter.Middleware("global"))
	authRepository := auth.NewRepository(app.Database.GORM)
	authService := auth.NewService(authRepository, app.Config.Security, app.Config.Mail.PublicBaseURL)
	authHandler := auth.NewHandler(authService, app.Config.CookieSecure)
	challengeHandler := challenge.NewHandler(challenge.New(app.Database.GORM, app.Config.Security.FlagHMACSecret))
	challengeFileHandler := challengefile.NewHandler(challengefile.New(app.Database.GORM, app.Storage))
	boardService := scoreboard.New(app.Database.GORM)
	boardHandler := scoreboard.NewHandler(boardService)
	assetService := asset.New(app.Database.GORM, app.Storage, app.Config.PublicBaseURL)
	assetHandler := asset.NewHandler(assetService)
	teamHandler := team.NewHandler(team.New(app.Database.GORM, app.Config.Security.ConfirmationTokenSecret, app.Config.Mail.PublicBaseURL, assetService))
	competitionHandler := competition.NewHandler(competition.New(app.Database.GORM, boardService))
	writeupHandler := writeup.NewHandler(writeup.New(app.Database.GORM))
	instanceService := instance.NewService(app.Database.GORM, app.Stream, app.Config.Runtime, app.Config.Security)
	instanceHandler := instance.NewHandler(instanceService, app.Database.GORM)
	registryCredentialHandler := registrycredential.NewHandler(registrycredential.New(app.Database.GORM, app.Config.Security.RegistryCredentialEncryptionKey), app.Config.Runtime.WorkerAPIToken)
	submissionHandler := submission.NewHandler(submission.New(app.Database.GORM, app.Cache.Client, app.Config.Security.FlagHMACSecret))
	appearanceHandler := appearance.NewHandler(appearance.New(app.Database.GORM))
	notificationHandler := notification.NewHandler(notification.New(app.Database.GORM, app.Cache.Client))
	progressionHandler := progression.NewHandler(progression.New(app.Database.GORM))
	learningHandler := learning.NewHandler(learning.New(app.Database.GORM))
	userHandler := user.NewHandler(user.New(app.Database.GORM))
	organizationHandler := organization.NewHandler(organization.New(app.Database.GORM))
	platformHandler := platformconfig.NewHandler(platformconfig.New(app.Database.GORM))
	hintHandler := hint.NewHandler(hint.New(app.Database.GORM))
	adminHandler := admin.NewHandler(admin.New(app.Database.GORM))
	optionalAuth := auth.Middleware(authService, false)
	requiredAuth := auth.Middleware(authService, true)
	router.GET("/health/live", func(c *gin.Context) { httpx.OK(c, map[string]string{"status": "live"}) })
	router.GET("/api/v1/health", func(c *gin.Context) { httpx.OK(c, map[string]string{"status": "ok"}) })
	router.GET("/health/ready", func(c *gin.Context) {
		readyCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := map[string]string{"postgres": "ok", "redis": "ok", "storage": "ok"}
		status := http.StatusOK
		if err := app.Database.Ready(readyCtx); err != nil {
			checks["postgres"] = "failed"
			status = http.StatusServiceUnavailable
		}
		if err := app.Cache.Ready(readyCtx); err != nil {
			checks["redis"] = "failed"
			status = http.StatusServiceUnavailable
		}
		if err := app.Storage.Ready(readyCtx); err != nil {
			checks["storage"] = "failed"
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, httpx.Envelope{Success: status == http.StatusOK, Data: checks, RequestID: httpx.RequestID(c)})
	})
	if app.Config.DocsEnabled {
		router.StaticFile("/openapi.yaml", "./openapi/openapi.yaml")
		router.StaticFile("/docs", "./openapi/index.html")
	}
	v1 := router.Group("/api/v1")
	v1.POST("/internal/runtime/registry-credentials/:id/lease", registryCredentialHandler.Lease)
	authRoutes := v1.Group("/auth")
	csrf := httpx.CSRFOrigins(app.Config.WebOrigins)
	authRoutes.POST("/register", rateLimiter.Middleware("auth-register"), authHandler.Register)
	authRoutes.POST("/login", rateLimiter.Middleware("auth-login"), authHandler.Login)
	authRoutes.POST("/verify-email", rateLimiter.Middleware("auth-verify-email"), authHandler.VerifyEmail)
	authRoutes.POST("/forgot-password", rateLimiter.Middleware("auth-forgot-password"), authHandler.ForgotPassword)
	authRoutes.POST("/reset-password", rateLimiter.Middleware("auth-reset-password"), authHandler.ResetPassword)
	authRoutes.POST("/confirm-email-change", rateLimiter.Middleware("auth-confirm-email-change"), authHandler.ConfirmEmailChange)
	authRoutes.POST("/refresh", csrf, authHandler.Refresh)
	authRoutes.POST("/logout", csrf, authHandler.Logout)
	authRoutes.POST("/logout-all", csrf, requiredAuth, authHandler.LogoutAll)
	authRoutes.GET("/me", requiredAuth, authHandler.Me)
	authRoutes.POST("/password", csrf, requiredAuth, authHandler.ChangePassword)
	authRoutes.POST("/verification-email", csrf, requiredAuth, rateLimiter.Middleware("auth-verification-email"), authHandler.ResendVerification)
	authRoutes.POST("/email/change", csrf, requiredAuth, rateLimiter.Middleware("auth-email-change"), authHandler.RequestEmailChange)
	public := v1.Group("")
	public.Use(optionalAuth)
	public.GET("/challenges", challengeHandler.List)
	public.GET("/challenges/:id", challengeHandler.Detail)
	public.GET("/competitions", competitionHandler.List)
	public.GET("/competitions/:id", competitionHandler.Detail)
	public.GET("/competitions/:id/scoreboard", boardHandler.Competition)
	public.GET("/leaderboard", boardHandler.Global)
	public.GET("/teams", teamHandler.List)
	public.GET("/teams/:id", teamHandler.Detail)
	public.GET("/writeups", writeupHandler.List)
	public.GET("/writeups/:id", writeupHandler.Detail)
	public.GET("/users/:id", userHandler.PublicProfile)
	public.GET("/organizations", organizationHandler.List)
	public.GET("/public/assets/manifest", assetHandler.PublicManifest)
	public.GET("/public/asset-content/:versionId", assetHandler.Content)
	public.GET("/public/themes/current", appearanceHandler.CurrentTheme)
	public.GET("/public/backgrounds/current", appearanceHandler.CurrentBackgrounds)
	public.GET("/public/bootstrap", platformHandler.Bootstrap)
	public.GET("/public/directions", platformHandler.Directions)
	public.GET("/learning/paths", learningHandler.List)
	public.GET("/learning/paths/:id", learningHandler.Detail)
	protected := v1.Group("")
	protected.Use(requiredAuth)
	protected.GET("/me/profile", userHandler.Me)
	protected.PATCH("/me/profile", userHandler.UpdateMe)
	protected.GET("/me/progression", progressionHandler.Me)
	protected.POST("/organizations", organizationHandler.Create)
	protected.POST("/organizations/:id/join", organizationHandler.Join)
	protected.POST("/teams", teamHandler.Create)
	protected.GET("/me/team", teamHandler.Manage)
	protected.PUT("/teams/:id", teamHandler.Update)
	protected.POST("/teams/:id/avatar", rateLimiter.Middleware("team-avatar-upload"), teamHandler.UploadAvatar)
	protected.POST("/teams/:id/join-requests", teamHandler.RequestJoin)
	protected.POST("/teams/:id/join-requests/:requestId/review", teamHandler.ReviewJoin)
	protected.POST("/teams/:id/invitations", teamHandler.Invite)
	protected.POST("/team-invitations/:invitationId/accept", teamHandler.AcceptInvitation)
	protected.POST("/teams/:id/transfer-captain", teamHandler.TransferCaptain)
	protected.DELETE("/teams/:id/members/:userId", teamHandler.RemoveMember)
	protected.POST("/teams/:id/leave", teamHandler.Leave)
	protected.POST("/teams/:id/announcements", teamHandler.PostAnnouncement)
	protected.POST("/competitions/:id/register", competitionHandler.Register)
	protected.POST("/writeups", writeupHandler.Create)
	protected.GET("/me/writeups", writeupHandler.Mine)
	protected.GET("/me/writeups/:id", writeupHandler.MineDetail)
	protected.PUT("/writeups/:id", writeupHandler.Update)
	protected.POST("/writeups/:id/submit-review", writeupHandler.SubmitReview)
	protected.POST("/writeups/:id/comments", writeupHandler.Comment)
	protected.POST("/writeups/:id/like", writeupHandler.Like)
	protected.PUT("/writeups/:id/favorite", writeupHandler.Favorite)
	protected.POST("/challenges/:id/submissions", rateLimiter.Middleware("flag-submit"), submissionHandler.Submit)
	protected.GET("/challenges/:id/submissions", submissionHandler.History)
	protected.GET("/challenges/:id/hints", hintHandler.List)
	protected.POST("/challenges/:id/hints/:index/unlock", hintHandler.Unlock)
	protected.GET("/challenges/:id/files/:fileId", challengeFileHandler.Download)
	instanceRoutes := protected.Group("/challenges/:id/instance")
	instanceRoutes.GET("/status", instanceHandler.Status)
	instanceRoutes.POST("/start", httpx.RequireIdempotencyKey(), instanceHandler.Start)
	instanceRoutes.POST("/restart", httpx.RequireIdempotencyKey(), instanceHandler.Restart)
	instanceRoutes.POST("/stop", httpx.RequireIdempotencyKey(), instanceHandler.Stop)
	instanceRoutes.POST("/reset", httpx.RequireIdempotencyKey(), instanceHandler.Reset)
	instanceRoutes.POST("/extend", instanceHandler.Extend)
	protected.GET("/notifications", notificationHandler.List)
	protected.PATCH("/notifications/:id/read", notificationHandler.Read)
	protected.POST("/notifications/read-all", notificationHandler.ReadAll)
	protected.GET("/events", notificationHandler.Events)
	adminRoutes := v1.Group("/admin")
	adminRoutes.Use(requiredAuth, auth.RequireAnyRole("super_admin", "site_admin", "visual_operator", "competition_admin", "challenge_author", "reviewer"))
	adminRoutes.GET("/dashboard", adminHandler.Dashboard)
	adminRoutes.GET("/users", auth.RequirePermission("user.read"), adminHandler.Users)
	adminRoutes.PATCH("/users/:id/status", auth.RequirePermission("user.ban"), adminHandler.SetUserStatus)
	adminRoutes.PATCH("/users/:id/roles", auth.RequirePermission("rbac.manage"), adminHandler.AssignRole)
	adminRoutes.GET("/challenges", auth.RequirePermission("challenge.read"), challengeHandler.AdminList)
	adminRoutes.GET("/challenges/:id", auth.RequirePermission("challenge.read"), challengeHandler.AdminDetail)
	adminRoutes.POST("/challenges", auth.RequirePermission("challenge.write"), challengeHandler.Create)
	adminRoutes.PUT("/challenges/:id", auth.RequirePermission("challenge.write"), challengeHandler.Update)
	adminRoutes.POST("/challenges/:id/publish", auth.RequirePermission("challenge.publish"), challengeHandler.Publish)
	adminRoutes.DELETE("/challenges/:id", auth.RequirePermission("challenge.write"), challengeHandler.Archive)
	adminRoutes.POST("/challenges/:id/files", auth.RequirePermission("challenge.write"), challengeFileHandler.Upload)
	adminRoutes.GET("/challenges/:id/files/:fileId", auth.RequirePermission("challenge.read"), challengeFileHandler.AdminDownload)
	adminRoutes.DELETE("/challenges/:id/files/:fileId", auth.RequirePermission("challenge.write"), challengeFileHandler.Delete)
	adminRoutes.GET("/competitions", auth.RequirePermission("competition.read"), competitionHandler.AdminList)
	adminRoutes.GET("/competitions/:id", auth.RequirePermission("competition.read"), competitionHandler.AdminDetail)
	adminRoutes.POST("/competitions", auth.RequirePermission("competition.write"), competitionHandler.Create)
	adminRoutes.PUT("/competitions/:id", auth.RequirePermission("competition.write"), competitionHandler.Update)
	adminRoutes.POST("/competitions/:id/status", auth.RequirePermission("competition.publish"), competitionHandler.SetStatus)
	adminRoutes.GET("/instances", auth.RequirePermission("instance.read"), instanceHandler.AdminList)
	adminRoutes.GET("/instances/:id", auth.RequirePermission("instance.read"), instanceHandler.AdminDetail)
	adminRoutes.POST("/instances/:id/stop", auth.RequirePermission("instance.manage"), httpx.RequireIdempotencyKey(), instanceHandler.AdminStop)
	adminRoutes.POST("/instances/:id/reset", auth.RequirePermission("instance.manage"), httpx.RequireIdempotencyKey(), instanceHandler.AdminReset)
	adminRoutes.GET("/instances/:id/logs", auth.RequirePermission("instance.read"), instanceHandler.AdminLogs)
	adminRoutes.GET("/runtime/workers", auth.RequirePermission("instance.read"), instanceHandler.AdminWorkers)
	adminRoutes.PATCH("/runtime/workers/:workerId/drain", auth.RequirePermission("instance.manage"), instanceHandler.AdminSetWorkerDrain)
	adminRoutes.GET("/registry-credentials", auth.RequirePermission("registry.read"), registryCredentialHandler.List)
	adminRoutes.POST("/registry-credentials", auth.RequirePermission("registry.manage"), registryCredentialHandler.Create)
	adminRoutes.PUT("/registry-credentials/:id", auth.RequirePermission("registry.manage"), registryCredentialHandler.Update)
	adminRoutes.GET("/submissions", auth.RequirePermission("submission.read"), adminHandler.Submissions)
	adminRoutes.GET("/anti-cheat", auth.RequirePermission("anticheat.read"), adminHandler.CheatCases)
	adminRoutes.PATCH("/anti-cheat/:id", auth.RequirePermission("anticheat.review"), adminHandler.ResolveCheatCase)
	adminRoutes.GET("/writeups", auth.RequirePermission("writeup.review"), writeupHandler.AdminList)
	adminRoutes.POST("/writeups/:id/review", auth.RequirePermission("writeup.review"), writeupHandler.Review)
	adminRoutes.GET("/announcements", auth.RequirePermission("announcement.write"), adminHandler.Announcements)
	adminRoutes.POST("/announcements", auth.RequirePermission("announcement.write"), adminHandler.CreateAnnouncement)
	adminRoutes.GET("/audit", auth.RequirePermission("audit.read"), adminHandler.Audit)
	adminRoutes.GET("/assets", auth.RequirePermission("asset.read"), assetHandler.List)
	adminRoutes.GET("/assets/:id", auth.RequirePermission("asset.read"), assetHandler.Get)
	adminRoutes.POST("/assets", auth.RequirePermission("asset.upload"), assetHandler.Create)
	adminRoutes.PUT("/assets/:id", auth.RequirePermission("asset.upload"), assetHandler.Update)
	adminRoutes.POST("/assets/:id/versions", auth.RequirePermission("asset.upload"), assetHandler.AddVersion)
	adminRoutes.POST("/assets/:id/publish", auth.RequirePermission("asset.publish"), assetHandler.Publish)
	adminRoutes.POST("/assets/:id/rollback", auth.RequirePermission("asset.rollback"), assetHandler.Rollback)
	adminRoutes.DELETE("/assets/:id", auth.RequirePermission("asset.archive"), assetHandler.Archive)
	adminRoutes.GET("/asset-categories", auth.RequirePermission("asset.read"), assetHandler.Categories)
	adminRoutes.POST("/asset-categories", auth.RequirePermission("asset.manage_taxonomy"), assetHandler.CreateCategory)
	adminRoutes.GET("/asset-tags", auth.RequirePermission("asset.read"), assetHandler.Tags)
	adminRoutes.POST("/asset-tags", auth.RequirePermission("asset.manage_taxonomy"), assetHandler.CreateTag)
	adminRoutes.GET("/appearance/slots", auth.RequirePermission("appearance.read"), assetHandler.Slots)
	adminRoutes.POST("/appearance/slots", auth.RequirePermission("appearance.write"), assetHandler.CreateSlot)
	adminRoutes.PUT("/appearance/slots/:id", auth.RequirePermission("appearance.write"), assetHandler.UpdateSlot)
	adminRoutes.GET("/appearance/backgrounds", auth.RequirePermission("appearance.read"), appearanceHandler.ListBackgrounds)
	adminRoutes.POST("/appearance/backgrounds", auth.RequirePermission("appearance.write"), appearanceHandler.SaveBackground)
	adminRoutes.PATCH("/appearance/backgrounds/:id", auth.RequirePermission("appearance.write"), appearanceHandler.SaveBackground)
	adminRoutes.POST("/appearance/backgrounds/:id/publish", auth.RequirePermission("appearance.publish"), appearanceHandler.PublishBackground)
	adminRoutes.POST("/appearance/backgrounds/:id/rollback", auth.RequirePermission("appearance.rollback"), appearanceHandler.RollbackBackground)
	adminRoutes.POST("/appearance/themes", auth.RequirePermission("appearance.write"), appearanceHandler.SaveTheme)
	adminRoutes.POST("/progression/schemes", auth.RequirePermission("progression.manage"), progressionHandler.CreateScheme)
	adminRoutes.GET("/learning/paths", auth.RequirePermission("progression.manage"), learningHandler.AdminList)
	adminRoutes.POST("/learning/paths", auth.RequirePermission("progression.manage"), learningHandler.Save)
	adminRoutes.PUT("/learning/paths/:id", auth.RequirePermission("progression.manage"), learningHandler.Save)
	adminRoutes.POST("/learning/paths/:id/publish", auth.RequirePermission("progression.manage"), learningHandler.Publish)
	adminRoutes.DELETE("/learning/paths/:id", auth.RequirePermission("progression.manage"), learningHandler.Archive)
	adminRoutes.GET("/platform/draft", auth.RequirePermission("platform.read"), platformHandler.Draft)
	adminRoutes.PUT("/platform/draft", auth.RequirePermission("platform.write"), platformHandler.SaveDraft)
	adminRoutes.POST("/platform/publish", auth.RequirePermission("platform.publish"), platformHandler.Publish)
	adminRoutes.GET("/directions", auth.RequirePermission("direction.read"), platformHandler.AdminDirections)
	adminRoutes.POST("/directions", auth.RequirePermission("direction.write"), platformHandler.SaveDirection)
	adminRoutes.PUT("/directions/:id", auth.RequirePermission("direction.write"), platformHandler.SaveDirection)
	adminRoutes.DELETE("/directions/:id", auth.RequirePermission("direction.archive"), platformHandler.ArchiveDirection)
	adminRoutes.POST("/score-events/:id/void", auth.RequirePermission("scoring.adjust"), boardHandler.VoidEvent)
	adminRoutes.POST("/score-events/adjust", auth.RequirePermission("scoring.adjust"), boardHandler.Adjust)
	adminRoutes.GET("/score-events", auth.RequirePermission("submission.read"), boardHandler.Events)
	adminRoutes.POST("/scoreboard/rebuild", auth.RequirePermission("scoring.rebuild"), boardHandler.Rebuild)
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			httpx.Fail(c, httpx.NewError(http.StatusNotFound, "ROUTE_NOT_FOUND", "接口不存在"))
			return
		}
		c.Status(http.StatusNotFound)
	})
	return router
}
