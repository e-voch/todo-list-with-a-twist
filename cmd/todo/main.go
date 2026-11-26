package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"todo/internal/cache"
	"todo/internal/metrics"
	"todo/internal/queue"
	"todo/internal/server"
	"todo/internal/storage"

	"github.com/redis/go-redis/v9"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	metricsAddr := flag.String("metrics-addr", ":9091", "address for /metrics; keep it off the public load balancer")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "todo")

	// One registry for the whole program, handed to every package that has
	// metrics, like the logger. The metrics server below serves all of it.
	reg := metrics.NewRegistry()

	store, err := storage.NewPostgresStore(os.Getenv("DATABASE_URL"), os.Getenv("DATABASE_READ_URL"))
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

	// Metrics get their own server, so the public port never serves them.
	go func() {
		logger.Info("metrics server starting", "addr", *metricsAddr)
		if err := http.ListenAndServe(*metricsAddr, metrics.NewHandler(reg)); err != nil {
			logger.Error("metrics server failed", "err", err)
			os.Exit(1)
		}
	}()

	logger.Info("server started", "addr", ":8080")
	if err := http.ListenAndServe(":8080", server.NewHandler(cached, producer, logger, reg)); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}
