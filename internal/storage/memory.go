package storage

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"todo/internal/task"
)

type MemoryStore struct {
	tasks map[uuid.UUID]task.Task
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tasks: make(map[uuid.UUID]task.Task)}
}

func (s *MemoryStore) List() ([]task.Task, error) {
	result := []task.Task{}
	for _, value := range s.tasks {
		result = append(result, value)
	}
	// Map order is random; UUIDv7s sort by creation time.
	slices.SortFunc(result, func(a, b task.Task) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return result, nil
}

func (s *MemoryStore) Get(id uuid.UUID) (task.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return task.Task{}, fmt.Errorf("task %s not found", id)
	}
	return t, nil
}

func (s *MemoryStore) Create(id uuid.UUID, title, description string) error {
	if _, ok := s.tasks[id]; ok {
		return nil
	}
	s.tasks[id] = task.Task{ID: id, Title: title, Description: description}
	slog.Info("task created", "id", id, "title", title, "description", description)
	return nil
}

func (s *MemoryStore) Update(id uuid.UUID, title, description string) error {
	s.tasks[id] = task.Task{ID: id, Title: title, Description: description}
	return nil
}

func (s *MemoryStore) Delete(id uuid.UUID) error {
	delete(s.tasks, id)
	return nil
}
