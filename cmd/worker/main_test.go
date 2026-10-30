package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"todo/internal/storage"
	"todo/internal/task"
)

// fakeStepsWriter records every SetRecommendedSteps call and returns err.
type fakeStepsWriter struct {
	calls map[uuid.UUID]string
	err   error
}

func newFakeStepsWriter(err error) *fakeStepsWriter {
	return &fakeStepsWriter{calls: map[uuid.UUID]string{}, err: err}
}

func (f *fakeStepsWriter) SetRecommendedSteps(id uuid.UUID, steps string) error {
	f.calls[id] = steps
	return f.err
}

func event(typ task.EventType) task.EventMessage {
	return task.EventMessage{Type: typ, Task: task.Task{ID: uuid.New(), Title: "Buy", Description: "Milk"}}
}

func Test_handle_SetsSteps(t *testing.T) {
	for _, typ := range []task.EventType{task.EventCreated, task.EventEdited} {
		t.Run(string(typ), func(t *testing.T) {
			store := newFakeStepsWriter(nil)
			e := event(typ)

			if err := handle(store, e); err != nil {
				t.Fatalf("handle: %v", err)
			}
			if got, ok := store.calls[e.Task.ID]; !ok || got == "" {
				t.Errorf("steps for %s = %q, want non-empty", e.Task.ID, got)
			}
		})
	}
}

func Test_handle_TaskGone(t *testing.T) {
	// The task was deleted before its event was handled: skip, don't retry forever.
	store := newFakeStepsWriter(fmt.Errorf("task: %w", storage.ErrNotFound))

	if err := handle(store, event(task.EventCreated)); err != nil {
		t.Errorf("handle = %v, want nil", err)
	}
}

func Test_handle_StoreError(t *testing.T) {
	storeErr := errors.New("connection refused")
	store := newFakeStepsWriter(storeErr)

	err := handle(store, event(task.EventEdited))
	if !errors.Is(err, storeErr) {
		t.Errorf("handle = %v, want %v", err, storeErr)
	}
}

func Test_handle_NoWrite(t *testing.T) {
	for _, typ := range []task.EventType{task.EventDeleted, "renamed"} {
		t.Run(string(typ), func(t *testing.T) {
			store := newFakeStepsWriter(errors.New("should not be called"))

			if err := handle(store, event(typ)); err != nil {
				t.Errorf("handle = %v, want nil", err)
			}
			if len(store.calls) != 0 {
				t.Errorf("SetRecommendedSteps called %d times, want 0", len(store.calls))
			}
		})
	}
}
