package storage

import (
	"testing"

	"github.com/google/uuid"

	"todo/internal/task"
)

func Test_Create(t *testing.T) {
	store := NewMemoryStore()
	id := uuid.Must(uuid.NewV7())

	if err := store.Create(id, "Title", "Description"); err != nil {
		t.Errorf("Create returned %v, want nil", err)
	}

	got := store.tasks[id]
	want := task.Task{ID: id, Title: "Title", Description: "Description"}
	if got != want {
		t.Errorf("after Create, task = %v, want %v", got, want)
	}
}

func Test_Create_Idempotent(t *testing.T) {
	testCreateIsIdempotent(t, NewMemoryStore())
}

func Test_Get(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	store := MemoryStore{
		tasks: map[uuid.UUID]task.Task{
			id: {ID: id, Title: "Title", Description: "Description"},
		},
	}

	tests := []struct {
		name    string
		id      uuid.UUID
		want    task.Task
		wantErr bool
	}{
		{"existing task", id, task.Task{ID: id, Title: "Title", Description: "Description"}, false},
		{"missing task", uuid.Must(uuid.NewV7()), task.Task{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Get(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get(%s) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Get(%s) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func Test_List(t *testing.T) {
	testListIsOldestFirst(t, NewMemoryStore())
}

func Test_Update(t *testing.T) {
	store := NewMemoryStore()
	id := createTask(t, store, "Title", "Description")

	err := store.Update(id, "New Title", "New Description")
	var wantErr error
	if err != wantErr {
		t.Errorf("Update returned %v, want %v", err, wantErr)
	}

	got := store.tasks[id]
	want := task.Task{ID: id, Title: "New Title", Description: "New Description"}
	if got != want {
		t.Errorf("after Update, task = %v, want %v", got, want)
	}
}

func Test_Delete(t *testing.T) {
	store := NewMemoryStore()
	id := createTask(t, store, "Title", "Description")

	err := store.Delete(id)
	var wantErr error
	if err != wantErr {
		t.Errorf("Delete returned %v, want %v", err, wantErr)
	}

	if _, ok := store.tasks[id]; ok {
		t.Errorf("Delete did not remove task %s from store", id)
	}
}
