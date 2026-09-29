package task

type Task struct {
	ID          int
	Title       string
	Description string
}

type Store interface {
	Create(title, description string) (id int, err error)
	Update(id int, title, description string) error
	Delete(id int) error
	Get(id int) (Task, error)
	List() ([]Task, error)
}
