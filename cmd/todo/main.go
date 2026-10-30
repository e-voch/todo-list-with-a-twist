package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"todo/internal/cache"
	"todo/internal/queue"
	"todo/internal/server"
	"todo/internal/storage"

	"github.com/redis/go-redis/v9"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	flag.Parse()

	store, err := storage.NewPostgresStore(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// An empty REDIS_ADDR means localhost:6379.
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
	defer rdb.Close()
	cached := cache.New(store, rdb, time.Minute)

	producer, err := queue.NewProducer(*brokers, *topic)
	if err != nil {
		slog.Error("failed to create producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	slog.Info("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", server.NewHandler(cached, producer)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
