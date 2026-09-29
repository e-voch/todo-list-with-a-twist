package storage

import (
	"fmt"
	"log/slog"

	"todo/internal/task"
)

type MemoryStore struct {
	tasks  map[int]task.Task
	nextID int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tasks: make(map[int]task.Task)}
}

func (s *MemoryStore) List() ([]task.Task, error) {
	result := []task.Task{}
	for _, value := range s.tasks {
		result = append(result, value)
	}
	return result, nil
}

func (s *MemoryStore) Get(id int) (task.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return task.Task{}, fmt.Errorf("task %d not found", id)
	}
	return t, nil
}

func (s *MemoryStore) Create(title, description string) (id int, err error) {
	s.nextID++
	id = s.nextID
	s.tasks[id] = task.Task{ID: id, Title: title, Description: description}
	slog.Info("task created", "id", id, "title", title, "description", description)
	return id, nil
}

func (s *MemoryStore) Update(id int, title, description string) error {
	s.tasks[id] = task.Task{ID: id, Title: title, Description: description}
	return nil
}

func (s *MemoryStore) Delete(id int) error {
	delete(s.tasks, id)
	return nil
}
