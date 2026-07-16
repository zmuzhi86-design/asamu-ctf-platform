package command

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"asamu.local/platform/api/internal/bootstrap"
	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/cache"
	"asamu.local/platform/api/internal/platform/database"
	"asamu.local/platform/api/internal/platform/queue"
	"asamu.local/platform/api/internal/platform/security"
	"asamu.local/platform/api/internal/platform/storage"
	"asamu.local/platform/api/internal/seed"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "serve":
		return serve(ctx)
	case "migrate":
		return migrate(args[1:], stdout)
	case "seed":
		return seedData(ctx, stdout)
	case "doctor":
		return doctor(ctx, stdout)
	case "admin":
		return admin(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		return usage(stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage(w io.Writer) error {
	fmt.Fprintln(w, "usage: asamu <serve|migrate|seed|doctor|admin>")
	fmt.Fprintln(w, "admin: list | create | reset-password | revoke-sessions")
	return nil
}

func serve(parent context.Context) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := bootstrap.Build(ctx)
	if err != nil {
		return err
	}
	defer app.Close()
	server := &http.Server{Addr: app.Config.HTTPAddr, Handler: app.Router, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 1 << 20}
	errCh := make(chan error, 1)
	go func() {
		app.Logger.Info("api_started", zap.String("addr", app.Config.HTTPAddr))
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func openDatabase() (config.Config, *database.Database, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, err
	}
	db, err := database.Open(cfg.Database, cfg.Environment)
	return cfg, db, err
}

func migrate(args []string, out io.Writer) error {
	_, db, err := openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	action := "up"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "up":
		err = database.Migrate(db.SQL)
	case "down":
		err = database.Rollback(db.SQL)
	case "status":
		err = database.Status(db.SQL)
	default:
		return fmt.Errorf("unknown migration command %q", action)
	}
	if err == nil {
		fmt.Fprintln(out, "migration command completed:", action)
	}
	return err
}

func seedData(ctx context.Context, out io.Writer) error {
	cfg, db, err := openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := seed.New(db.GORM, cfg.Security.FlagHMACSecret).Run(ctx); err != nil {
		return err
	}
	fmt.Fprintln(out, "seed completed")
	return nil
}

func doctor(ctx context.Context, out io.Writer) error {
	cfg, db, err := openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.Ready(checkCtx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	redisClient, err := cache.Open(cfg.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer redisClient.Close()
	if err := redisClient.Ready(checkCtx); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	store, err := storage.Open(cfg.Storage)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	if err := store.Ready(checkCtx); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	stream := queue.NewStream(redisClient.Client, cfg.Redis.Stream, cfg.Redis.ConsumerGroup, "doctor")
	if err := stream.Ensure(checkCtx); err != nil {
		return fmt.Errorf("runtime stream: %w", err)
	}
	fmt.Fprintln(out, "[OK] configuration")
	fmt.Fprintln(out, "[OK] postgres")
	fmt.Fprintln(out, "[OK] redis")
	fmt.Fprintln(out, "[OK] storage")
	fmt.Fprintln(out, "[OK] runtime stream")
	return nil
}

func admin(ctx context.Context, args []string, out, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("admin subcommand is required")
	}
	_, db, err := openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	switch args[0] {
	case "list":
		return adminList(ctx, db.GORM, out)
	case "create":
		return adminCreate(ctx, db.GORM, args[1:], out, stderr)
	case "reset-password":
		return adminResetPassword(ctx, db.GORM, args[1:], out, stderr)
	case "revoke-sessions":
		return adminRevokeSessions(ctx, db.GORM, args[1:], out, stderr)
	default:
		return fmt.Errorf("unknown admin command %q", args[0])
	}
}

func adminList(ctx context.Context, db *gorm.DB, out io.Writer) error {
	var rows []struct {
		ID                      uuid.UUID
		Username, Email, Status string
	}
	if err := db.WithContext(ctx).Table("users").Select("users.id, users.username, users.email, users.status").Joins("JOIN user_roles ur ON ur.user_id=users.id").Joins("JOIN roles r ON r.id=ur.role_id").Where("r.key IN ? AND users.deleted_at IS NULL", []string{"super_admin", "site_admin"}).Group("users.id").Order("users.username").Scan(&rows).Error; err != nil {
		return err
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tEMAIL\tSTATUS\tID")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", row.Username, row.Email, row.Status, row.ID)
	}
	return w.Flush()
}

func adminCreate(ctx context.Context, db *gorm.DB, args []string, out, stderr io.Writer) error {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "administrator username")
	email := fs.String("email", "", "administrator email")
	displayName := fs.String("display-name", "", "display name")
	password := fs.String("password", "", "password; generated when omitted")
	role := fs.String("role", "super_admin", "role key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*username) == "" || !strings.Contains(*email, "@") {
		return errors.New("--username and a valid --email are required")
	}
	generated := false
	if *password == "" {
		var err error
		*password, err = randomPassword()
		if err != nil {
			return err
		}
		generated = true
	}
	hash, err := security.HashPassword(*password)
	if err != nil {
		return err
	}
	userID := uuid.New()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var selected models.Role
		if err := tx.Where("key=?", *role).First(&selected).Error; err != nil {
			return fmt.Errorf("role %q not found: %w", *role, err)
		}
		now := time.Now().UTC()
		user := models.User{ID: userID, Username: strings.TrimSpace(*username), Email: strings.ToLower(strings.TrimSpace(*email)), PasswordHash: hash, Status: "active", TokenVersion: 1, MustChangePassword: generated, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		profile := models.UserProfile{UserID: userID, DisplayName: strings.TrimSpace(*displayName), Skills: []byte("[]"), Privacy: []byte("{}"), UpdatedAt: now}
		if err := tx.Create(&profile).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserRole{UserID: userID, RoleID: selected.ID}).Error
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "administrator created:", userID)
	if generated {
		fmt.Fprintln(out, "temporary password (shown once):", *password)
	}
	return nil
}

func adminResetPassword(ctx context.Context, db *gorm.DB, args []string, out, stderr io.Writer) error {
	fs := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "administrator username")
	password := fs.String("password", "", "new password; generated when omitted")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	generated := false
	if *password == "" {
		var err error
		*password, err = randomPassword()
		if err != nil {
			return err
		}
		generated = true
	}
	hash, err := security.HashPassword(*password)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username=? AND deleted_at IS NULL", *username).First(&user).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&user).Updates(map[string]any{"password_hash": hash, "password_changed_at": now, "must_change_password": generated, "token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", user.ID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "admin_password_reset"}).Error; err != nil {
			return err
		}
		fmt.Fprintln(out, "password reset and sessions revoked for:", user.Username)
		if generated {
			fmt.Fprintln(out, "temporary password (shown once):", *password)
		}
		return nil
	})
}

func adminRevokeSessions(ctx context.Context, db *gorm.DB, args []string, out, stderr io.Writer) error {
	fs := flag.NewFlagSet("admin revoke-sessions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "administrator username")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username=? AND deleted_at IS NULL", *username).First(&user).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&user).Updates(map[string]any{"token_version": gorm.Expr("token_version+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		result := tx.Model(&models.RefreshToken{}).Where("user_id=? AND revoked_at IS NULL", user.ID).Update("revoked_at", now)
		if result.Error != nil {
			return result.Error
		}
		if err := tx.Model(&models.UserSession{}).Where("user_id=? AND revoked_at IS NULL", user.ID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "admin_revoke"}).Error; err != nil {
			return err
		}
		fmt.Fprintf(out, "revoked %d refresh session(s) for %s\n", result.RowsAffected, *username)
		return nil
	})
}

func randomPassword() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
