package storage

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"slices"
	"testing"

	"todo/db/migrations"
	"todo/internal/task"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var testURL string

func TestMain(m *testing.M) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:18",
		postgres.WithDatabase("todos"),
		postgres.WithUsername("todo"),
		postgres.WithPassword("todo"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	testURL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		testcontainers.TerminateContainer(ctr)
		log.Fatalf("connection string: %v", err)
	}

	if err := migrate(ctx, testURL); err != nil {
		testcontainers.TerminateContainer(ctr)
		log.Fatalf("migrate: %v", err)
	}

	code := m.Run()

	if err := testcontainers.TerminateContainer(ctr); err != nil {
		log.Printf("terminate container: %v", err)
	}
	os.Exit(code)
}

func migrate(ctx context.Context, url string) error {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return err
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func newTestStore(t *testing.T) *PostgresStore {
	t.Helper()

	store, err := NewPostgresStore(testURL)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if _, err := store.db.Exec(`TRUNCATE tasks RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return store
}

func Test_Postgres_List_Empty(t *testing.T) {
	store := newTestStore(t)

	tasks, err := store.List(100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("want 0 tasks, got %d", len(tasks))
	}
}

func Test_Postgres_List(t *testing.T) {
	store := newTestStore(t)

	id1 := uuid.New()
	id2 := uuid.New()

	err := store.Create(id1, "buy milk", "2 litres")
	if err != nil {
		t.Fatalf("Create task 1: %v", err)
	}
	err = store.Create(id2, "walk dog", "30 minutes")
	if err != nil {
		t.Fatalf("Create task 2: %v", err)
	}

	tasks, err := store.List(100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != id1 {
		t.Errorf("tasks[0].ID: want %s, got %s", id1, tasks[0].ID)
	}
	if tasks[0].Title != "buy milk" {
		t.Errorf("tasks[0].Title: want %q, got %q", "buy milk", tasks[0].Title)
	}
	if tasks[1].ID != id2 {
		t.Errorf("tasks[1].ID: want %s, got %s", id2, tasks[1].ID)
	}
	if tasks[1].Title != "walk dog" {
		t.Errorf("tasks[1].Title: want %q, got %q", "walk dog", tasks[1].Title)
	}
}

func Test_Postgres_SetRecommendedSteps(t *testing.T) {
	store := newTestStore(t)
	id := createTask(t, store, "buy milk", "2 litres")

	if err := store.SetRecommendedSteps(id, "go to the shop"); err != nil {
		t.Fatalf("SetRecommendedSteps: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RecommendedSteps != "go to the shop" {
		t.Errorf("RecommendedSteps: want %q, got %q", "go to the shop", got.RecommendedSteps)
	}
}

func Test_Postgres_SetRecommendedSteps_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.SetRecommendedSteps(uuid.New(), "steps")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func Test_Postgres_Create_Idempotent(t *testing.T) {
	store := newTestStore(t)
	id := createTask(t, store, "Title", "Description")

	// A redelivered "created" event must keep the first task, not add another.
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

	list, err := store.List(100, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("after second Create, List returned %d tasks, want 1", len(list))
	}
}

func Test_Postgres_List_OldestFirst(t *testing.T) {
	store := newTestStore(t)
	var want []uuid.UUID
	for _, title := range []string{"First", "Second", "Third"} {
		want = append(want, createTask(t, store, title, "Description"))
	}

	list, err := store.List(100, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != len(want) {
		t.Fatalf("List returned %d tasks, want %d", len(list), len(want))
	}
	for i, got := range list {
		if got.ID != want[i] {
			t.Errorf("List(100,0)[%d].ID = %s, want %s", i, got.ID, want[i])
		}
	}
}

// createTask stores a task under a fresh UUIDv7 and returns its ID.
func createTask(t *testing.T, store *PostgresStore, title, description string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, title, description); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	return id
}

func Test_pagination_List(t *testing.T) {
	store := newTestStore(t)
	createTask(t, store, "buy milk", "2 litres")
	createTask(t, store, "walk dog", "30 minutes")
	id3 := createTask(t, store, "Call Euni", "Whatsapp")
	id4 := createTask(t, store, "Gym", "30 minutes at the gym")

	want := []task.Task{
		{ID: id3, Title: "Call Euni", Description: "Whatsapp"},
		{ID: id4, Title: "Gym", Description: "30 minutes at the gym"},
	}

	got, err := store.List(2, 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if !slices.Equal(got, want) {
		t.Errorf("List(2, 2) = %v, want %v", got, want)
	}
}

func Test_Postgres_Count(t *testing.T) {
	store := newTestStore(t)
	createTask(t, store, "One", "Description")
	createTask(t, store, "Two", "Description")

	got, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 2 {
		t.Errorf("Count = %d, want 2", got)
	}
}
