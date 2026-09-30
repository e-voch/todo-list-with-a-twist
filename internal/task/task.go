package task

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Store interface {
	Create(title, description string) (id int, err error)
	Update(id int, title, description string) error
	Delete(id int) error
	Get(id int) (Task, error)
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
