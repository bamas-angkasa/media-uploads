// cmd/migrate is a standalone CLI for running goose migrations manually.
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down
//	go run ./cmd/migrate status
//	go run ./cmd/migrate create <name> sql
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/yourusername/media-share/config"
	migrations "github.com/yourusername/media-share/migrations"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "set dialect: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"status"}
	}

	command := args[0]
	cmdArgs := args[1:]

	if err := goose.RunWithOptions(command, db, ".", cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "goose %s: %v\n", command, err)
		os.Exit(1)
	}
}
