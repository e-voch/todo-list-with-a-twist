package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"todo/internal/task"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var ErrNotFound = errors.New("task not found")

// maxConns caps how many connections one app instance opens to Postgres.
// Without a cap, every request waiting on the database opens another one, and
// Postgres (a process and memory per connection) runs out of memory under
// load. With the cap, extra requests wait in the app instead. Keep
// maxConns × the most app instances (the scaling max) under Postgres's
// max_connections (100).
const maxConns = 10

type PostgresStore struct {
	db     *sql.DB // primary: every write, and Get
	readDB *sql.DB // replica: List and Count; nil when there is no replica
}

// NewPostgresStore connects to the primary at url. readURL is the replica;
// pass "" to send every read to the primary.
func NewPostgresStore(url, readURL string) (*PostgresStore, error) {
	db, err := openPool(url)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	store := &PostgresStore{db: db}

	if readURL != "" {
		// No Ping here: a missing replica must not stop the app from
		// starting. Reads fall back to the primary until it answers.
		readDB, err := openPool(readURL)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("open postgres replica: %w", err)
		}
		store.readDB = readDB
	}
	return store, nil
}

// openPool creates a connection pool. It doesn't connect yet; the first
// query does.
func openPool(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxConns)
	// Keep the connections open between requests instead of reconnecting;
	// the default keeps only 2 idle.
	db.SetMaxIdleConns(maxConns)
	return db, nil
}

func (p *PostgresStore) Close() error {
	if p.readDB != nil {
		_ = p.readDB.Close()
	}
	return p.db.Close()
}

// reader picks where List and Count read from: the replica if it has caught
// up with the primary, otherwise the primary.
func (p *PostgresStore) reader() *sql.DB {
	// Part 1: no replica configured (worker, tests).
	if p.readDB == nil {
		return p.db
	}

	// Part 2: how far is the primary? Every write that has finished is at or
	// before this position.
	var primaryLSN string
	err := p.db.QueryRow(`SELECT pg_current_wal_lsn()::text`).Scan(&primaryLSN)
	if err != nil {
		return p.db
	}

	// Part 3: has the replica replayed that far? NULL (not a replica) counts
	// as no.
	var caughtUp bool
	err = p.readDB.QueryRow(
		`SELECT COALESCE(pg_last_wal_replay_lsn() >= $1::pg_lsn, false)`,
		primaryLSN,
	).Scan(&caughtUp)
	if err != nil || !caughtUp {
		return p.db
	}
	return p.readDB
}

// likePattern turns a search query into an ILIKE pattern that matches it
// anywhere. %, _ and \ in the query are escaped so they match literally; an
// empty query becomes "%%", which matches every task.
func likePattern(query string) string {
	return "%" + likeEscaper.Replace(query) + "%"
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (p *PostgresStore) List(query string, limit, offset int) ([]task.Task, error) {
	rows, err := p.reader().Query(`SELECT uuid, title, description, COALESCE(recommended_steps, '') FROM tasks WHERE title ILIKE $1 OR description ILIKE $1 ORDER BY id LIMIT $2 OFFSET $3`, likePattern(query), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tasks := []task.Task{}
	for rows.Next() {
		var t task.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.RecommendedSteps); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

func (p *PostgresStore) Get(id uuid.UUID) (task.Task, error) {
	var t task.Task
	err := p.db.QueryRow(`SELECT uuid, title, description, COALESCE(recommended_steps, '') FROM tasks WHERE uuid = $1`, id).
		Scan(&t.ID, &t.Title, &t.Description, &t.RecommendedSteps)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("task %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("get task %s: %w", id, err)
	}
	return t, nil
}

func (p *PostgresStore) Create(id uuid.UUID, title, description string) error {
	_, err := p.db.Exec(`INSERT INTO tasks (uuid, title, description) VALUES ($1, $2, $3) ON CONFLICT (uuid) DO NOTHING`, id, title, description)
	if err != nil {
		return fmt.Errorf("create task %s: %w", id, err)
	}
	return nil
}

func (p *PostgresStore) Update(id uuid.UUID, title, description string) error {
	res, err := p.db.Exec(`UPDATE tasks SET title = $2, description = $3, recommended_steps = NULL WHERE uuid = $1`, id, title, description)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
	}
	return checkFound(res, id)
}

func (p *PostgresStore) SetRecommendedSteps(id uuid.UUID, steps string) error {
	res, err := p.db.Exec(`UPDATE tasks SET recommended_steps = $2 WHERE uuid = $1`, id, steps)
	if err != nil {
		return fmt.Errorf("set recommended steps for task %s: %w", id, err)
	}
	return checkFound(res, id)
}

func (p *PostgresStore) Delete(id uuid.UUID) error {
	res, err := p.db.Exec(`DELETE FROM tasks WHERE uuid = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task %s: %w", id, err)
	}
	return checkFound(res, id)
}

func checkFound(res sql.Result, id uuid.UUID) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task %s: %w", id, ErrNotFound)
	}
	return nil
}

func (p *PostgresStore) Count(query string) (int, error) {
	var c int
	err := p.reader().QueryRow(`SELECT COUNT(*) FROM tasks WHERE title ILIKE $1 OR description ILIKE $1`, likePattern(query)).
		Scan(&c)
	if err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}
	return c, nil
}
