package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"todo/internal/task"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(url string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (p *PostgresStore) Close() error {
	return p.db.Close()
}

func (p *PostgresStore) List() ([]task.Task, error) {
	rows, err := p.db.Query(`SELECT uuid, title, description FROM tasks ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []task.Task{}
	for rows.Next() {
		var t task.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description); err != nil {
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
	err := p.db.QueryRow(`SELECT uuid, title, description FROM tasks WHERE uuid = $1`, id).
		Scan(&t.ID, &t.Title, &t.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("task %s not found", id)
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
	res, err := p.db.Exec(`UPDATE tasks SET title = $2, description = $3 WHERE uuid = $1`, id, title, description)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
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
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}
