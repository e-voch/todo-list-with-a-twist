package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"todo/internal/queue"
	"todo/internal/storage"
	"todo/internal/task"
)

const loremIpsum = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."

type StepsWriter interface {
	SetRecommendedSteps(id uuid.UUID, steps string) error
}

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	group := flag.String("group", "todo-worker", "kafka consumer group id")
	flag.Parse()

	store, err := storage.NewPostgresStore(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	consumer, err := queue.NewConsumer(*brokers, *group, *topic)
	if err != nil {
		slog.Error("failed to create consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("worker started", "topic", *topic, "group", *group)
	for ctx.Err() == nil {
		msg, err := consumer.Read(500 * time.Millisecond)
		if err != nil {
			slog.Error("read failed", "err", err)
			continue
		}
		if msg == nil {
			continue // no event within the timeout
		}

		for {
			err := handle(store, msg.Event)
			if err == nil {
				break
			}
			slog.Error("handle failed, retrying", "type", msg.Event.Type, "id", msg.Event.Task.ID, "err", err)
			select {
			case <-ctx.Done():
				slog.Info("worker stopped before event was handled; it will be redelivered")
				return
			case <-time.After(time.Second):
			}
		}

		if err := consumer.Commit(msg); err != nil {
			slog.Error("commit failed", "id", msg.Event.Task.ID, "err", err)
		}
	}
	slog.Info("worker stopped")
}

func handle(store StepsWriter, e task.EventMessage) error {
	switch e.Type {
	case task.EventCreated, task.EventEdited:
		err := store.SetRecommendedSteps(e.Task.ID, loremIpsum)
		if errors.Is(err, storage.ErrNotFound) {
			slog.Info("task no longer exists, skipping", "type", e.Type, "id", e.Task.ID)
			return nil
		}
		if err != nil {
			return err
		}
		slog.Info("recommended steps set", "type", e.Type, "id", e.Task.ID)
	case task.EventDeleted:
		slog.Info("task deleted, nothing to do", "id", e.Task.ID)
	default:
		slog.Warn("unknown event type, skipping", "type", e.Type, "id", e.Task.ID)
	}
	return nil
}
