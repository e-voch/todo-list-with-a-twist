package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"todo/internal/storage"
	"todo/internal/task"
)

func Test_process_CountsOutcomes(t *testing.T) {
	m := newWorkerMetrics(prometheus.NewRegistry())

	// Two created events done, one edited for a task that's gone, one delete.
	ok := newFakeStepsWriter(nil)
	gone := newFakeStepsWriter(fmt.Errorf("task: %w", storage.ErrNotFound))
	for _, c := range []struct {
		store StepsWriter
		typ   task.EventType
	}{
		{ok, task.EventCreated},
		{ok, task.EventCreated},
		{gone, task.EventEdited},
		{ok, task.EventDeleted},
	} {
		if err := process(c.store, discard, m, event(c.typ)); err != nil {
			t.Fatalf("process %s: %v", c.typ, err)
		}
	}

	for _, c := range []struct {
		typ, outcome string
		want         float64
	}{
		{"created", outcomeOK, 2},
		{"edited", outcomeSkipped, 1},
		{"deleted", outcomeSkipped, 1},
	} {
		if got := testutil.ToFloat64(m.events.WithLabelValues(c.typ, c.outcome)); got != c.want {
			t.Errorf("events{type=%q,outcome=%q} = %v, want %v", c.typ, c.outcome, got, c.want)
		}
	}
	if got := testutil.CollectAndCount(m.handleErrors); got != 0 {
		t.Errorf("handle_errors has %d series, want 0", got)
	}
	if got := testutil.CollectAndCount(m.handleDuration); got != 3 {
		t.Errorf("handle_duration has %d series, want 3 (one per type)", got)
	}
}

func Test_process_CountsErrors(t *testing.T) {
	// The worker retries a failing event, so every attempt counts as an error
	// and none as a handled event: that's what a stuck worker looks like.
	m := newWorkerMetrics(prometheus.NewRegistry())
	store := newFakeStepsWriter(errors.New("connection refused"))
	e := event(task.EventEdited)

	for range 3 {
		if err := process(store, discard, m, e); err == nil {
			t.Fatal("process = nil, want the store's error")
		}
	}

	if got := testutil.ToFloat64(m.handleErrors.WithLabelValues("edited")); got != 3 {
		t.Errorf("handle_errors{type=edited} = %v, want 3", got)
	}
	if got := testutil.CollectAndCount(m.events); got != 0 {
		t.Errorf("events has %d series, want 0", got)
	}
}

func Test_typeLabel(t *testing.T) {
	// Event types come from Kafka messages; an unexpected one must not become
	// a label value of its own.
	for in, want := range map[task.EventType]string{
		task.EventCreated: "created",
		task.EventEdited:  "edited",
		task.EventDeleted: "deleted",
		"renamed":         "unknown",
		"":                "unknown",
	} {
		if got := typeLabel(in); got != want {
			t.Errorf("typeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
