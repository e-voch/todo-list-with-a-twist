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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/redis/go-redis/v9"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "todo")

	// One registry for the whole program, handed to every package that has
	// metrics, like the logger. /metrics serves all of it.
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),                                       // memory, goroutines, GC
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // CPU, open files
	)

	store, err := storage.NewPostgresStore(os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("close store", "err", err)
		}
	}()

	// An empty REDIS_ADDR means localhost:6379.
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("close redis", "err", err)
		}
	}()
	cached := cache.New(store, rdb, time.Minute, logger, reg)

	producer, err := queue.NewProducer(*brokers, *topic)
	if err != nil {
		logger.Error("failed to create producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	logger.Info("server started", "addr", ":8080")
	if err := http.ListenAndServe(":8080", server.NewHandler(cached, producer, logger, reg)); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}
