package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"todo/internal/storage"
	"todo/internal/task"
)

func Test_notFoundHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/does-not-exits", nil)
	rec := httptest.NewRecorder()

	notFoundHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func Test_faviconHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()

	faviconHandler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func Test_homeHandler_Create(t *testing.T) {
	store := storage.NewMemoryStore()
	pub := &fakePublisher{}
	req := httptest.NewRequest(http.MethodGet, "/?ftitle=Buy&fdescription=Milk", nil)
	rec := httptest.NewRecorder()

	homeHandler(store, pub)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}

	if len(pub.events) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.events))
	}
	got := pub.events[0]
	if got.Task.ID.Version() != 7 {
		t.Errorf("published task ID %s is version %d, want a UUIDv7", got.Task.ID, got.Task.ID.Version())
	}
	want := task.EventMessage{
		Type: task.EventCreated,
		Task: task.Task{ID: got.Task.ID, Title: "Buy", Description: "Milk"},
	}
	if got != want {
		t.Errorf("published event = %v, want %v", pub.events[0], want)
	}

	saved, err := store.Get(got.Task.ID)
	if err != nil {
		t.Fatalf("Get(%s) returned error: %v", got.Task.ID, err)
	}
	if saved != want.Task {
		t.Errorf("saved task = %v, want %v", saved, want.Task)
	}
}

func Test_homeHandler_List(t *testing.T) {
	store := storage.NewMemoryStore()
	if err := store.Create(uuid.Must(uuid.NewV7()), "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	homeHandler(store, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Buy") || !strings.Contains(body, "Milk") {
		t.Errorf("body does not contain the task; body = %s", body)
	}
}

func Test_deleteHandler(t *testing.T) {
	store := storage.NewMemoryStore()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	pub := &fakePublisher{}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/delete/?id=%s", id), nil)
	rec := httptest.NewRecorder()

	deleteHandler(store, pub)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}

	if len(pub.events) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.events))
	}
	want := task.EventMessage{Type: task.EventDeleted, Task: task.Task{ID: id}}
	if pub.events[0] != want {
		t.Errorf("published event = %v, want %v", pub.events[0], want)
	}

	if _, err := store.Get(id); err == nil {
		t.Errorf("after delete, Get(%s) returned nil error, want error", id)
	}
}

func Test_deleteHandler_InvalidID(t *testing.T) {
	pub := &fakePublisher{}
	req := httptest.NewRequest(http.MethodGet, "/delete/?id=abc", nil)
	rec := httptest.NewRecorder()

	deleteHandler(storage.NewMemoryStore(), pub)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}
	if len(pub.events) != 0 {
		t.Errorf("published %d events for an invalid id, want 0", len(pub.events))
	}
}

func Test_editHandler_InvalidID(t *testing.T) {
	store := storage.NewMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/edit/?id=abc", nil)
	rec := httptest.NewRecorder()

	editHandler(store, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}
}

func Test_editHandler_Form(t *testing.T) {
	store := storage.NewMemoryStore()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/edit/?id=%s", id), nil)
	rec := httptest.NewRecorder()

	editHandler(store, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// The form inputs should be pre-filled with the task's current values.
	body := rec.Body.String()
	for _, want := range []string{`value="Buy"`, `value="Milk"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %s; body = %s", want, body)
		}
	}
}

func Test_editHandler_Save(t *testing.T) {
	store := storage.NewMemoryStore()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	pub := &fakePublisher{}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/edit/?id=%s&ftitle=Sell&fdescription=Bread", id), nil)
	rec := httptest.NewRecorder()

	editHandler(store, pub)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}

	if len(pub.events) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.events))
	}
	want := task.EventMessage{
		Type: task.EventEdited,
		Task: task.Task{ID: id, Title: "Sell", Description: "Bread"},
	}
	if pub.events[0] != want {
		t.Errorf("published event = %v, want %v", pub.events[0], want)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != want.Task {
		t.Errorf("after edit, Get(%s) = %v, want %v", id, got, want.Task)
	}
}

// failingStore is a Store whose every method fails, for testing how
// handlers react to storage errors.
type failingStore struct{}

var errStoreDown = errors.New("store down")

func (failingStore) Create(id uuid.UUID, title, description string) error { return errStoreDown }
func (failingStore) Update(id uuid.UUID, title, description string) error { return errStoreDown }
func (failingStore) Delete(id uuid.UUID) error                            { return errStoreDown }
func (failingStore) Get(id uuid.UUID) (task.Task, error)                  { return task.Task{}, errStoreDown }
func (failingStore) List() ([]task.Task, error)                           { return nil, errStoreDown }

// fakePublisher records published events instead of sending them to Kafka.
type fakePublisher struct {
	events []task.EventMessage
}

func (p *fakePublisher) Publish(e task.EventMessage) error {
	p.events = append(p.events, e)
	return nil
}

// failingPublisher is a Publisher that always fails, like a Kafka outage.
type failingPublisher struct{}

var errQueueDown = errors.New("queue down")

func (failingPublisher) Publish(e task.EventMessage) error { return errQueueDown }

func Test_handlers_StoreError(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	pub := &fakePublisher{}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		url     string
	}{
		{"home create", homeHandler(failingStore{}, pub), "/?ftitle=Buy&fdescription=Milk"},
		{"home list", homeHandler(failingStore{}, pub), "/"},
		{"delete", deleteHandler(failingStore{}, pub), "/delete/?id=" + id.String()},
		{"edit save", editHandler(failingStore{}, pub), "/edit/?id=" + id.String() + "&ftitle=Sell&fdescription=Bread"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
			}
			if loc := rec.Header().Get("Location"); loc != "" {
				t.Errorf("redirected to %q on store error, want no redirect", loc)
			}
			if len(pub.events) != 0 {
				t.Errorf("published %d events after a failed write, want 0", len(pub.events))
			}
		})
	}
}

// A publish failure must not fail the request: the task is already saved.
func Test_handlers_PublishError(t *testing.T) {
	store := storage.NewMemoryStore()
	id := uuid.Must(uuid.NewV7())
	if err := store.Create(id, "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		url     string
	}{
		{"home create", homeHandler(store, failingPublisher{}), "/?ftitle=New&fdescription=Task"},
		{"edit save", editHandler(store, failingPublisher{}), "/edit/?id=" + id.String() + "&ftitle=Sell&fdescription=Bread"},
		{"delete", deleteHandler(store, failingPublisher{}), "/delete/?id=" + id.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
		})
	}

	// After create, edit and delete of the original, only the new task is left.
	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "New" {
		t.Errorf("store has %v, want only the task titled %q", tasks, "New")
	}
}

func Test_editHandler_MissingTask(t *testing.T) {
	store := storage.NewMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/edit/?id="+uuid.Must(uuid.NewV7()).String(), nil)
	rec := httptest.NewRecorder()

	editHandler(store, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "<form") {
		t.Errorf("rendered the edit form for a missing task")
	}
}
