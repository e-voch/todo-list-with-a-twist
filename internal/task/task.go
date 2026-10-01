package task

import "github.com/google/uuid"

type Task struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
}

type Store interface {
	// Create stores a new task under id. If a task with that id already
	// exists, Create does nothing, so a redelivered event is harmless.
	Create(id uuid.UUID, title, description string) error
	Update(id uuid.UUID, title, description string) error
	Delete(id uuid.UUID) error
	Get(id uuid.UUID) (Task, error)
	// List returns tasks oldest first.
	List() ([]Task, error)
}

type EventType string

const (
	EventCreated EventType = "created"
	EventEdited  EventType = "edited"
	EventDeleted EventType = "deleted"
)

type EventMessage struct {
	Type EventType `json:"type"`
	Task Task      `json:"task"`
}
