package storage

import (
	"testing"

	"github.com/google/uuid"

	"todo/internal/task"
)

// createTask stores a task under a fresh UUIDv7 and returns its ID.
func createTask(t *testing.T, store task.Store, title, description string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, title, description); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	return id
}

// testCreateIsIdempotent checks that creating the same ID twice keeps the
// first task and does not add a second one, as a redelivered event would.
func testCreateIsIdempotent(t *testing.T, store task.Store) {
	t.Helper()
	id := createTask(t, store, "Title", "Description")

	if err := store.Create(id, "Other", "Other description"); err != nil {
		t.Fatalf("second Create returned error: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := task.Task{ID: id, Title: "Title", Description: "Description"}
	if got != want {
		t.Errorf("after second Create, Get(%s) = %v, want %v", id, got, want)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("after second Create, List returned %d tasks, want 1", len(list))
	}
}

// testListIsOldestFirst checks that List returns tasks in creation order.
func testListIsOldestFirst(t *testing.T, store task.Store) {
	t.Helper()
	var want []uuid.UUID
	for _, title := range []string{"First", "Second", "Third"} {
		want = append(want, createTask(t, store, title, "Description"))
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != len(want) {
		t.Fatalf("List returned %d tasks, want %d", len(list), len(want))
	}
	for i, got := range list {
		if got.ID != want[i] {
			t.Errorf("List()[%d].ID = %s, want %s", i, got.ID, want[i])
		}
	}
}
