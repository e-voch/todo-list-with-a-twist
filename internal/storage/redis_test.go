package storage

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"todo/internal/task"
)

func newTestRedisStore(t *testing.T) *RedisStore {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7")
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	addr, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get redis address: %v", err)
	}

	store, err := NewRedisStore(addr)
	if err != nil {
		t.Fatalf("failed to create redis store: %v", err)
	}
	t.Cleanup(func() { store.client.Close() })

	return store
}

func Test_List_Redis(t *testing.T) {
	store := newTestRedisStore(t)

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

func Test_Get_Redis(t *testing.T) {
	store := newTestRedisStore(t)

	id, err := store.Create("Title", "Description")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	tests := []struct {
		name    string
		id      int
		want    task.Task
		wantErr bool
	}{
		{"existing task", id, task.Task{ID: id, Title: "Title", Description: "Description"}, false},
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

func Test_Update_Redis(t *testing.T) {
	store := newTestRedisStore(t)

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

func Test_Delete_Redis(t *testing.T) {
	store := newTestRedisStore(t)

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

	if _, err := store.Get(id); err == nil {
		t.Errorf("after Delete, Get(%d) returned nil error, want error", id)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("after Delete, List returned %d tasks, want 1", len(list))
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

func Test_NewRedisStore_Unreachable(t *testing.T) {
	// Nothing listens on port 1, so the connection is refused.
	_, err := NewRedisStore("localhost:1")
	if err == nil {
		t.Fatal("NewRedisStore with unreachable address returned nil error, want error")
	}
}

func Test_List_Redis_Empty(t *testing.T) {
	store := newTestRedisStore(t)

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

func Test_Redis_ClosedClient(t *testing.T) {
	store := newTestRedisStore(t)
	store.client.Close()

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
				t.Errorf("%s on closed client returned nil error, want error", tt.name)
			}
		})
	}
}
