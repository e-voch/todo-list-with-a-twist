package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo/internal/queue"
	"todo/internal/task"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "kafka bootstrap servers")
	topic := flag.String("topic", "tasks", "kafka topic for task events")
	group := flag.String("group", "todo-worker", "kafka consumer group id")
	flag.Parse()

	consumer, err := queue.NewConsumer(*brokers, *group, *topic)
	if err != nil {
		slog.Error("failed to create consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// ctx is cancelled on Ctrl+C or SIGTERM, which ends the loop below.
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

		// Retry until the event is handled, so it is never skipped and later
		// events never overtake it.
		for {
			err := handle(msg.Event)
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

// handle reacts to one event. The task is already saved by the web server;
// this is where follow-up work, such as asking an LLM for a suggestion, goes.
// It must be safe to repeat, because Kafka may deliver an event more than once.
func handle(e task.EventMessage) error {
	switch e.Type {
	case task.EventCreated, task.EventEdited, task.EventDeleted:
		slog.Info(fmt.Sprintf("task %s", e.Type), "id", e.Task.ID, "title", e.Task.Title)
	default:
		// Retrying will not help with a type we do not know, so log and skip it.
		slog.Warn("unknown event type, skipping", "type", e.Type, "id", e.Task.ID)
	}
	return nil
}
