package task

import "github.com/google/uuid"

type Task struct {
	ID               uuid.UUID `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	RecommendedSteps string    `json:"recommended_steps,omitempty"`
}

type Store interface {
	// Create stores a new task under id. If a task with that id already
	// exists, Create does nothing, so a redelivered event is harmless.
	Create(id uuid.UUID, title, description string) error
	Update(id uuid.UUID, title, description string) error
	Delete(id uuid.UUID) error
	Get(id uuid.UUID) (Task, error)
	// List returns up to limit tasks, oldest first, skipping the first offset.
	List(limit, offset int) ([]Task, error)
	// Count returns the total number of tasks.
	Count() (int, error)
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
