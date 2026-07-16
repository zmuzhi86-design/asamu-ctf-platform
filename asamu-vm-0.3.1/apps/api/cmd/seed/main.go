package main

import (
	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/platform/database"
	"asamu.local/platform/api/internal/seed"
	"context"
	"fmt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	db, err := database.Open(cfg.Database, cfg.Environment)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := seed.New(db.GORM, cfg.Security.FlagHMACSecret).Run(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("seed completed; administrator is only created when DEV_ADMIN_* is configured")
}
