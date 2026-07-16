package main

import (
	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/platform/database"
	"fmt"
	"os"
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
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "up":
		err = database.Migrate(db.SQL)
	case "down":
		err = database.Rollback(db.SQL)
	case "status":
		err = database.Status(db.SQL)
	default:
		err = fmt.Errorf("unknown migration command %s", command)
	}
	if err != nil {
		panic(err)
	}
	fmt.Println("migration command completed:", command)
}
