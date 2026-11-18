package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"todo/internal/cache"
	"todo/internal/metrics"
	"todo/internal/queue"
	"todo/internal/storage"
	"todo/internal/task"
)

const loremIpsum = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."

type StepsWriter interface {
	SetRecommendedSteps(id uuid.UUID, steps string) error
}

// invalidatingWriter clears the app's page cache after each successful write,
// so the home page shows new steps straight away.
type invalidatingWriter struct {
	StepsWriter
	rdb    *redis.Client
	logger *slog.Logger
}

func (w invalidatingWriter) SetRecommendedSteps(id uuid.UUID, steps string) error {
	if err := w.StepsWriter.SetRecommendedSteps(id, steps); err != nil {
		return err
	}
	cache.Invalidate(w.rdb, w.logger)
	return nil
}

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	group := flag.String("group", "todo-worker", "kafka consumer group id")
	metricsAddr := flag.String("metrics-addr", ":9091", "address for /metrics")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "worker")

	reg := metrics.NewRegistry()
	m := newWorkerMetrics(reg)
	go func() {
		logger.Info("metrics server starting", "addr", *metricsAddr)
		if err := http.ListenAndServe(*metricsAddr, metrics.NewHandler(reg)); err != nil {
			logger.Error("metrics server failed", "err", err)
			os.Exit(1)
		}
	}()

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
	writer := invalidatingWriter{StepsWriter: store, rdb: rdb, logger: logger}

	consumer, err := queue.NewConsumer(*brokers, *group, *topic)
	if err != nil {
		logger.Error("failed to create consumer", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			logger.Error("close consumer", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("worker started", "topic", *topic, "group", *group)
	for ctx.Err() == nil {
		msg, err := consumer.Read(500 * time.Millisecond)
		if err != nil {
			m.kafkaErrors.WithLabelValues("read").Inc()
			logger.Error("read failed", "err", err)
			continue
		}
		if msg == nil {
			continue // no event within the timeout
		}

		for {
			err := process(writer, logger, m, msg.Event)
			if err == nil {
				break
			}
			logger.Error("handle failed, retrying", "type", msg.Event.Type, "task_id", msg.Event.Task.ID, "err", err)
			select {
			case <-ctx.Done():
				logger.Info("worker stopped before event was handled; it will be redelivered")
				return
			case <-time.After(time.Second):
			}
		}

		if err := consumer.Commit(msg); err != nil {
			m.kafkaErrors.WithLabelValues("commit").Inc()
			logger.Error("commit failed", "task_id", msg.Event.Task.ID, "err", err)
		}
	}
	logger.Info("worker stopped")
}

// handle does the work for one event. On success it says whether there was
// anything to do (outcomeOK) or not (outcomeSkipped).
func handle(store StepsWriter, logger *slog.Logger, e task.EventMessage) (string, error) {
	switch e.Type {
	case task.EventCreated, task.EventEdited:
		err := store.SetRecommendedSteps(e.Task.ID, loremIpsum)
		if errors.Is(err, storage.ErrNotFound) {
			logger.Info("task no longer exists, skipping", "type", e.Type, "task_id", e.Task.ID)
			return outcomeSkipped, nil
		}
		if err != nil {
			return "", err
		}
		logger.Info("recommended steps set", "type", e.Type, "task_id", e.Task.ID)
		return outcomeOK, nil
	case task.EventDeleted:
		logger.Info("task deleted, nothing to do", "task_id", e.Task.ID)
		return outcomeSkipped, nil
	default:
		logger.Warn("unknown event type, skipping", "type", e.Type, "task_id", e.Task.ID)
		return outcomeSkipped, nil
	}
}
