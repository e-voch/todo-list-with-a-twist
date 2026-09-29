package storage

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"todo/internal/task"
)

func newTestStore(t *testing.T) *SqliteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSqliteStore(path)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.db.Close() })
	return store
}

func Test_List_Sqlite(t *testing.T) {
	store := newTestStore(t)

	id1, err := store.Create("Title", "Description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	id2, err := store.Create("Title2", "Description2")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d tasks, want %d", len(got), 2)
	}

	byID := map[int]task.Task{}
	for _, task := range got {
		byID[task.ID] = task
	}

	want1 := task.Task{ID: id1, Title: "Title", Description: "Description"}
	if byID[id1] != want1 {
		t.Errorf("List task %d = %v, want %v", id1, byID[id1], want1)
	}

	want2 := task.Task{ID: id2, Title: "Title2", Description: "Description2"}
	if byID[id2] != want2 {
		t.Errorf("List task %d = %v, want %v", id2, byID[id2], want2)
	}
}

func Test_Get_Sqlite(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Create("Title", "Description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	tests := []struct {
		name    string
		id      int
		want    task.Task
		wantErr error
	}{
		{"existing task", id, task.Task{ID: id, Title: "Title", Description: "Description"}, nil},
		{"missing task", 99, task.Task{}, sql.ErrNoRows},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Get(tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Get(%d) error = %v, want %v", tt.id, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Get(%d) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func Test_Update_Sqlite(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Create("Title", "Description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	otherID, err := store.Create("Other", "Other description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := store.Update(id, "New Title", "New Description"); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := task.Task{ID: id, Title: "New Title", Description: "New Description"}
	if got != want {
		t.Errorf("after Update, Get(%d) = %v, want %v", id, got, want)
	}

	other, err := store.Get(otherID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	wantOther := task.Task{ID: otherID, Title: "Other", Description: "Other description"}
	if other != wantOther {
		t.Errorf("Update changed another task: Get(%d) = %v, want %v", otherID, other, wantOther)
	}
}

func Test_Delete_Sqlite(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Create("Title", "Description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	otherID, err := store.Create("Other", "Other description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := store.Delete(id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err = store.Get(id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after Delete, Get(%d) error = %v, want %v", id, err, sql.ErrNoRows)
	}

	other, err := store.Get(otherID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	wantOther := task.Task{ID: otherID, Title: "Other", Description: "Other description"}
	if other != wantOther {
		t.Errorf("Delete changed another task: Get(%d) = %v, want %v", otherID, other, wantOther)
	}
}

func Test_NewSqliteStore_BadPath(t *testing.T) {
	_, err := NewSqliteStore(filepath.Join(t.TempDir(), "missing", "test.db"))
	if err == nil {
		t.Fatal("NewSqliteStore with missing directory returned nil error, want error")
	}
}

func Test_List_Sqlite_Empty(t *testing.T) {
	store := newTestStore(t)

	got, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got == nil {
		t.Errorf("List returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("List returned %d tasks, want 0", len(got))
	}
}

func Test_List_Sqlite_ScanError(t *testing.T) {
	store := newTestStore(t)

	// A NULL title can't be scanned into a string, so List must return an error.
	if _, err := store.db.Exec("INSERT INTO tasks (title, description) VALUES (NULL, 'Description')"); err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	if _, err := store.List(); err == nil {
		t.Fatal("List with NULL title returned nil error, want error")
	}
}

func Test_Sqlite_ClosedDB(t *testing.T) {
	store := newTestStore(t)
	store.db.Close()

	tests := []struct {
		name string
		call func() error
	}{
		{"List", func() error { _, err := store.List(); return err }},
		{"Get", func() error { _, err := store.Get(1); return err }},
		{"Create", func() error { _, err := store.Create("Title", "Description"); return err }},
		{"Update", func() error { return store.Update(1, "Title", "Description") }},
		{"Delete", func() error { return store.Delete(1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Errorf("%s on closed database returned nil error, want error", tt.name)
			}
		})
	}
}
