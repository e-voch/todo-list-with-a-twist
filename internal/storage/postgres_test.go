package storage

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"

	"todo/db/migrations"

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
		postgres.WithDatabase("todo"),
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

	tasks, err := store.List()
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

	tasks, err := store.List()
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
