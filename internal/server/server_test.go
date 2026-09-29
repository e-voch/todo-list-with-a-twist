package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	req := httptest.NewRequest(http.MethodGet, "/?ftitle=Buy&fdescription=Milk", nil)
	rec := httptest.NewRecorder()

	homeHandler(store)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Store has %d tasks, want 1", len(tasks))
	}
	if tasks[0].Title != "Buy" || tasks[0].Description != "Milk" {
		t.Errorf("created task = %v, want title %q, description %q", tasks[0], "Buy", "Milk")
	}
}

func Test_homeHandler_List(t *testing.T) {
	store := storage.NewMemoryStore()
	if _, err := store.Create("Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	homeHandler(store)(rec, req)

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
	id, err := store.Create("Buy", "Milk")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/delete/?id=%d", id), nil)
	rec := httptest.NewRecorder()

	deleteHandler(store)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}
	if _, err := store.Get(id); err == nil {
		t.Errorf("after delete, Get(%d) returned nil error, want error", id)
	}
}

func Test_deleteHandler_InvalidID(t *testing.T) {
	store := storage.NewMemoryStore()
	id, err := store.Create("Buy", "Milk")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/delete/?id=abc", nil)
	rec := httptest.NewRecorder()

	deleteHandler(store)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}
	if _, err := store.Get(id); err != nil {
		t.Errorf("task %d was deleted, want it kept: Get error = %v", id, err)
	}
}

func Test_editHandler_InvalidID(t *testing.T) {
	store := storage.NewMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/edit/?id=abc", nil)
	rec := httptest.NewRecorder()

	editHandler(store)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}
}

func Test_editHandler_Form(t *testing.T) {
	store := storage.NewMemoryStore()
	id, err := store.Create("Buy", "Milk")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/edit/?id=%d", id), nil)
	rec := httptest.NewRecorder()

	editHandler(store)(rec, req)

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
	id, err := store.Create("Buy", "Milk")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/edit/?id=%d&ftitle=Sell&fdescription=Bread", id), nil)
	rec := httptest.NewRecorder()

	editHandler(store)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want %q", loc, "/")
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := task.Task{ID: id, Title: "Sell", Description: "Bread"}
	if got != want {
		t.Errorf("after edit, Get(%d) = %v, want %v", id, got, want)
	}
}

// failingStore is a Store whose every method fails, for testing how
// handlers react to storage errors.
type failingStore struct{}

var errStoreDown = errors.New("store down")

func (failingStore) Create(title, description string) (int, error)  { return 0, errStoreDown }
func (failingStore) Update(id int, title, description string) error { return errStoreDown }
func (failingStore) Delete(id int) error                            { return errStoreDown }
func (failingStore) Get(id int) (task.Task, error)                  { return task.Task{}, errStoreDown }
func (failingStore) List() ([]task.Task, error)                     { return nil, errStoreDown }

func Test_handlers_StoreError(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		url     string
	}{
		{"home create", homeHandler(failingStore{}), "/?ftitle=Buy&fdescription=Milk"},
		{"home list", homeHandler(failingStore{}), "/"},
		{"delete", deleteHandler(failingStore{}), "/delete/?id=1"},
		{"edit save", editHandler(failingStore{}), "/edit/?id=1&ftitle=Sell&fdescription=Bread"},
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
		})
	}
}

func Test_editHandler_MissingTask(t *testing.T) {
	store := storage.NewMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/edit/?id=99", nil)
	rec := httptest.NewRecorder()

	editHandler(store)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "<form") {
		t.Errorf("rendered the edit form for a missing task")
	}
}
