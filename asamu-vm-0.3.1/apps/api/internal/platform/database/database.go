package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"asamu.local/platform/api/internal/config"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Database struct {
	GORM *gorm.DB
	SQL  *sql.DB
}

func Open(cfg config.Database, environment string) (*Database, error) {
	gormLogger := logger.Default.LogMode(logger.Warn)
	if environment == "production" {
		gormLogger = logger.Default.LogMode(logger.Error)
	}
	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{Logger: gormLogger, TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Database{GORM: db, SQL: sqlDB}, nil
}

func (d *Database) Close() error { return d.SQL.Close() }
func (d *Database) Ready(ctx context.Context) error {
	if err := d.SQL.PingContext(ctx); err != nil {
		return err
	}
	var schemaReady bool
	err := d.SQL.QueryRowContext(ctx, `SELECT
to_regclass('public.users') IS NOT NULL AND
to_regclass('public.challenges') IS NOT NULL AND
to_regclass('public.learning_paths') IS NOT NULL AND
to_regclass('public.learning_stages') IS NOT NULL AND
to_regclass('public.learning_stage_challenges') IS NOT NULL AND
to_regclass('public.runtime_worker_nodes') IS NOT NULL AND
to_regclass('public.competition_roster_members') IS NOT NULL AND
to_regclass('public.team_competition_solve_claims') IS NOT NULL`).Scan(&schemaReady)
	if err != nil {
		return fmt.Errorf("check database schema: %w", err)
	}
	if !schemaReady {
		return fmt.Errorf("database schema is incomplete; run asamu migrate up")
	}
	return nil
}
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}
func Rollback(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Down(db, "migrations")
}
func Status(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Status(db, "migrations")
}

func Transaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}
