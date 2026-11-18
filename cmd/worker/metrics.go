package main

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"todo/internal/task"
)

// What happened to an event that was handled without error.
const (
	outcomeOK      = "ok"      // the work was done
	outcomeSkipped = "skipped" // nothing to do: task gone, delete, or unknown type
)

// workerMetrics are the worker's own metrics. A stuck worker shows as
// handle_errors climbing while events stays flat: it retries the same
// event every second.
type workerMetrics struct {
	events         *prometheus.CounterVec
	handleErrors   *prometheus.CounterVec
	handleDuration *prometheus.HistogramVec
	kafkaErrors    *prometheus.CounterVec
}

// newWorkerMetrics creates the worker's metrics and registers them with reg.
func newWorkerMetrics(reg prometheus.Registerer) *workerMetrics {
	m := &workerMetrics{
		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_events_total",
			Help: "Events handled, by event type and outcome (ok, skipped).",
		}, []string{"type", "outcome"}),
		handleErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_handle_errors_total",
			Help: "Failed attempts to handle an event, by event type. Each retry counts.",
		}, []string{"type"}),
		handleDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "worker_handle_duration_seconds",
			Help:    "How long one attempt to handle an event took, by event type.",
			Buckets: prometheus.DefBuckets, // 5ms up to 10s
		}, []string{"type"}),
		kafkaErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_kafka_errors_total",
			Help: "Kafka errors, by operation (read, commit).",
		}, []string{"op"}),
	}
	reg.MustRegister(m.events, m.handleErrors, m.handleDuration, m.kafkaErrors)
	return m
}

// process makes one attempt at handling e and records how it went.
func process(store StepsWriter, logger *slog.Logger, m *workerMetrics, e task.EventMessage) error {
	typ := typeLabel(e.Type)
	start := time.Now()
	outcome, err := handle(store, logger, e)
	m.handleDuration.WithLabelValues(typ).Observe(time.Since(start).Seconds())
	if err != nil {
		m.handleErrors.WithLabelValues(typ).Inc()
		return err
	}
	m.events.WithLabelValues(typ, outcome).Inc()
	return nil
}

// typeLabel keeps the type label to the known event types. The type comes
// from the message, so without this any value a producer sent would become
// a new series for Prometheus to store.
func typeLabel(t task.EventType) string {
	switch t {
	case task.EventCreated, task.EventEdited, task.EventDeleted:
		return string(t)
	default:
		return "unknown"
	}
}
