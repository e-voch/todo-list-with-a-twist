package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"todo/internal/server"
	"todo/internal/storage"
	"todo/internal/task"
)

func main() {
	storeType := flag.String("store", "sqlite", "storage backend to use: \"sqlite\", \"memory\", or \"redis\"")
	dbPath := flag.String("db", "tasks.db", "path to the sqlite database file (used when -store=sqlite)")
	redisAddr := flag.String("redis-addr", "localhost:6379", "redis server address (used when -store=redis)")
	flag.Parse()

	var store task.Store
	switch *storeType {
	case "sqlite":
		sqliteStore, err := storage.NewSqliteStore(*dbPath)
		if err != nil {
			slog.Error("failed to open store", "err", err)
			os.Exit(1)
		}
		store = sqliteStore
	case "memory":
		store = storage.NewMemoryStore()
	case "redis":
		redisStore, err := storage.NewRedisStore(*redisAddr)
		if err != nil {
			slog.Error("failed to open store", "err", err)
			os.Exit(1)
		}
		store = redisStore
	default:
		slog.Error("unknown store type", "store", *storeType)
		os.Exit(1)
	}

	slog.Info("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", server.NewHandler(store)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
