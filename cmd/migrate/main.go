package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"todo/db/migrations"
)

func main() {
	flag.Parse()
	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		slog.Error("ping database", "err", err)
		os.Exit(1)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithVerbose(true))
	if err != nil {
		slog.Error("create migration provider", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	switch command {
	case "up":
		if _, err := provider.Up(ctx); err != nil {
			slog.Error("migrate up", "err", err)
			os.Exit(1)
		}
	case "down":
		if _, err := provider.Down(ctx); err != nil {
			slog.Error("migrate down", "err", err)
			os.Exit(1)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			slog.Error("migrate status", "err", err)
			os.Exit(1)
		}

		for _, s := range statuses {
			fmt.Printf("%-40s %-8s %s\n", s.Source.Path, s.State, s.AppliedAt)

		}
	default:
		slog.Error("unknown command", "command", command)
		os.Exit(1)
	}
}
