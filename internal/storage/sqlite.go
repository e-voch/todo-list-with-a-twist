package storage

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"todo/internal/task"
)

type SqliteStore struct {
	db *sql.DB
}

func NewSqliteStore(path string) (*SqliteStore, error) {
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if _, err := database.Exec("CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY, title TEXT, description TEXT)"); err != nil {
		database.Close()
		return nil, err
	}

	return &SqliteStore{db: database}, nil
}

func (s *SqliteStore) List() ([]task.Task, error) {
	results := []task.Task{}

	rows, err := s.db.Query("SELECT id, title, description FROM tasks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t task.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *SqliteStore) Get(id int) (task.Task, error) {
	var t task.Task
	row := s.db.QueryRow("SELECT id, title, description FROM tasks WHERE id = ?", id)
	if err := row.Scan(&t.ID, &t.Title, &t.Description); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

func (s *SqliteStore) Create(title, description string) (id int, err error) {
	result, err := s.db.Exec("INSERT INTO tasks (title, description) VALUES (?, ?)", title, description)
	if err != nil {
		return 0, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(lastID), nil
}

func (s *SqliteStore) Update(id int, title, description string) error {
	_, err := s.db.Exec("UPDATE tasks SET title = ?, description = ? WHERE id = ?", title, description, id)
	return err
}

func (s *SqliteStore) Delete(id int) error {
	_, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}
