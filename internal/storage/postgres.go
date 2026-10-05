package storage

import (
	"database/sql"
	"fmt"

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

// TODO: WRITE THESE FUNCTIONS

// func (p *PostgresStore) List() ([]task.Task, error) {
// 	db, err := p.db.Query(`select `)
// }

// func (p *PostgresStore) Get(id uuid.UUID) (task.Task, error) {
// }

// func (p *PostgresStore) Create(id uuid.UUID, title, description string) error {
// }

// func (p *PostgresStore) Update(id uuid.UUID, title, description string) error {

// func (p *PostgresStore) Delete(id uuid.UUID) error {
// }
