package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"todo/internal/queue"
	"todo/internal/server"
	"todo/internal/storage"
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

	producer, err := queue.NewProducer(*brokers, *topic)
	if err != nil {
		slog.Error("failed to create producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	slog.Info("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", server.NewHandler(store, producer)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
