package storage

import (
	"testing"

	"todo/internal/task"
)

func Test_Create(t *testing.T) {
	store := MemoryStore{tasks: make(map[int]task.Task)}
	got, err := store.Create("Title", "Description")
	wantedId := 1
	var wantedErr error
	if got != wantedId {
		t.Errorf("Create returned %d, want %d", got, wantedId)
	}
	if err != wantedErr {
		t.Errorf("Create returned %s, want %s", err, wantedErr)
	}
}

func Test_Get(t *testing.T) {
	store := MemoryStore{
		tasks: map[int]task.Task{
			1: {ID: 1, Title: "Title", Description: "Description"},
		},
	}

	tests := []struct {
		name    string
		id      int
		want    task.Task
		wantErr bool
	}{
		{"existing task", 1, task.Task{ID: 1, Title: "Title", Description: "Description"}, false},
		{"missing task", 99, task.Task{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Get(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get(%d) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Get(%d) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func Test_List(t *testing.T) {
	store := MemoryStore{
		tasks: map[int]task.Task{
			1: {ID: 1, Title: "Title", Description: "Description"},
			2: {ID: 2, Title: "Test1", Description: "Description1"},
			3: {ID: 3, Title: "Test2", Description: "Description2"},
		},
	}
	items, err := store.List()
	got := len(items)
	want := 3
	var wantErr error
	if got != want {
		t.Errorf("List returned %d, want %d", got, want)
	}
	if err != wantErr {
		t.Errorf("List returned %s, want %s", err, wantErr)
	}
}

func Test_Update(t *testing.T) {
	store := MemoryStore{
		tasks: map[int]task.Task{
			1: {ID: 1, Title: "Title", Description: "Description"},
		},
	}

	err := store.Update(1, "New Title", "New Description")
	var wantErr error
	if err != wantErr {
		t.Errorf("Update returned %v, want %v", err, wantErr)
	}

	got := store.tasks[1]
	want := task.Task{ID: 1, Title: "New Title", Description: "New Description"}
	if got != want {
		t.Errorf("after Update, task = %v, want %v", got, want)
	}
}

func Test_Delete(t *testing.T) {
	store := MemoryStore{
		tasks: map[int]task.Task{
			1: {ID: 1, Title: "Title", Description: "Description"},
		},
	}
	err := store.Delete(1)

	var wantErr error
	if err != wantErr {
		t.Errorf("Delete returned %v, want %v", err, wantErr)
	}

	if _, ok := store.tasks[1]; ok {
		t.Errorf("Delete did not remove task 1 from store")
	}
}
