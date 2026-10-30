package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
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

func Test_limitURLLength(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := limitURLLength(next)

	tests := []struct {
		name string
		url  string
		want int
	}{
		{"at limit", "/" + strings.Repeat("a", maxURLLength-1), http.StatusOK},
		{"over limit", "/" + strings.Repeat("a", maxURLLength), http.StatusRequestURITooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func Test_homeHandler_Create(t *testing.T) {
	store := newFakeStore()
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
	store := newFakeStore()
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
	store := newFakeStore()
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

	deleteHandler(newFakeStore(), pub)(rec, req)

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
	store := newFakeStore()
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
	store := newFakeStore()
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
	store := newFakeStore()
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

// fakeStore is an in-memory Store for handler tests. Like PostgresStore,
// Create ignores an existing ID, and Get, Update and Delete return
// storage.ErrNotFound for a missing task.
type fakeStore struct {
	tasks []task.Task // oldest first
}

func newFakeStore() *fakeStore { return &fakeStore{} }

func (s *fakeStore) find(id uuid.UUID) int {
	return slices.IndexFunc(s.tasks, func(t task.Task) bool { return t.ID == id })
}

func (s *fakeStore) Create(id uuid.UUID, title, description string) error {
	if s.find(id) < 0 {
		s.tasks = append(s.tasks, task.Task{ID: id, Title: title, Description: description})
	}
	return nil
}

func (s *fakeStore) Update(id uuid.UUID, title, description string) error {
	i := s.find(id)
	if i < 0 {
		return storage.ErrNotFound
	}
	s.tasks[i].Title, s.tasks[i].Description, s.tasks[i].RecommendedSteps = title, description, ""
	return nil
}

func (s *fakeStore) Delete(id uuid.UUID) error {
	i := s.find(id)
	if i < 0 {
		return storage.ErrNotFound
	}
	s.tasks = slices.Delete(s.tasks, i, i+1)
	return nil
}

func (s *fakeStore) Get(id uuid.UUID) (task.Task, error) {
	i := s.find(id)
	if i < 0 {
		return task.Task{}, storage.ErrNotFound
	}
	return s.tasks[i], nil
}

// matching returns the tasks whose title or description contains query,
// ignoring case, like the ILIKE in PostgresStore.
func (s *fakeStore) matching(query string) []task.Task {
	q := strings.ToLower(query)
	var out []task.Task
	for _, t := range s.tasks {
		if strings.Contains(strings.ToLower(t.Title), q) || strings.Contains(strings.ToLower(t.Description), q) {
			out = append(out, t)
		}
	}
	return out
}

func (s *fakeStore) List(query string, limit, offset int) ([]task.Task, error) {
	tasks := s.matching(query)
	// Clamp both ends like SQL does: an offset past the end gives an empty page.
	start := min(offset, len(tasks))
	end := min(start+limit, len(tasks))
	return slices.Clone(tasks[start:end]), nil
}

func (s *fakeStore) Count(query string) (int, error) { return len(s.matching(query)), nil }

// failingStore is a Store whose every method fails, for testing how
// handlers react to storage errors.
type failingStore struct{}

var errStoreDown = errors.New("store down")

func (failingStore) Create(id uuid.UUID, title, description string) error { return errStoreDown }
func (failingStore) Update(id uuid.UUID, title, description string) error { return errStoreDown }
func (failingStore) Delete(id uuid.UUID) error                            { return errStoreDown }
func (failingStore) Get(id uuid.UUID) (task.Task, error)                  { return task.Task{}, errStoreDown }
func (failingStore) List(query string, limit, offset int) ([]task.Task, error) {
	return nil, errStoreDown
}
func (failingStore) Count(query string) (int, error) { return 0, errStoreDown }

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
	store := newFakeStore()
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
	tasks, err := store.List("", 100, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "New" {
		t.Errorf("store has %v, want only the task titled %q", tasks, "New")
	}
}

func Test_editHandler_MissingTask(t *testing.T) {
	store := newFakeStore()
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
func Test_homeHandler_Pagination(t *testing.T) {
	store := newFakeStore()
	for i := range pageSize + 1 {
		if err := store.Create(uuid.Must(uuid.NewV7()), fmt.Sprintf("Task %02d", i), "Description"); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}

	tests := []struct {
		name    string
		url     string
		wantIn  []string
		wantOut []string
	}{
		{"first page", "/", []string{"Task 00", "?page=2", `<span aria-current="page">1</span>`}, []string{"Task 10", "?page=0"}},
		{"second page", "/?page=2", []string{"Task 10", "?page=1", `<span aria-current="page">2</span>`}, []string{"Task 00", "?page=3"}},
		{"bad page falls back to 1", "/?page=abc", []string{"Task 00"}, []string{"Task 10"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			homeHandler(store, &fakePublisher{})(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))

			body := rec.Body.String()
			for _, s := range tt.wantIn {
				if !strings.Contains(body, s) {
					t.Errorf("body missing %q", s)
				}
			}
			for _, s := range tt.wantOut {
				if strings.Contains(body, s) {
					t.Errorf("body unexpectedly contains %q", s)
				}
			}
		})
	}
}

func Test_homeHandler_Search(t *testing.T) {
	store := newFakeStore()
	for i := range pageSize + 1 {
		if err := store.Create(uuid.Must(uuid.NewV7()), fmt.Sprintf("Buy item %d", i), "Shop"); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}
	if err := store.Create(uuid.Must(uuid.NewV7()), "Walk", "Dog"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	tests := []struct {
		name     string
		url      string
		contains []string
		excludes []string
	}{
		{
			name:     "filters and keeps query in pagination",
			url:      "/?q=buy",
			contains: []string{"Buy item 0", `value="buy"`, `href="/?page=2&q=buy"`, `<a href="/?page=2&amp;q=buy">2</a>`},
			excludes: []string{"Walk"},
		},
		{
			name:     "second page of results",
			url:      "/?q=buy&page=2",
			contains: []string{"Buy item 10", `href="/?page=1&q=buy"`},
			excludes: []string{"Walk", "Buy item 0 "},
		},
		{
			name:     "no matches",
			url:      "/?q=xyz",
			contains: []string{"No tasks match"},
			excludes: []string{"Buy", "Walk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			homeHandler(store, &fakePublisher{})(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, s := range tt.contains {
				if !strings.Contains(body, s) {
					t.Errorf("body does not contain %q; body = %s", s, body)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(body, s) {
					t.Errorf("body contains %q, want it filtered out", s)
				}
			}
		})
	}
}

// A page past the end of the results redirects to the last page of results,
// not of all tasks.
func Test_homeHandler_SearchPastEnd(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(uuid.Must(uuid.NewV7()), "Buy", "Milk"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/?q=buy&page=5", nil)
	rec := httptest.NewRecorder()

	homeHandler(store, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc, want := rec.Header().Get("Location"), "/?page=1&q=buy"; loc != want {
		t.Errorf("redirect = %q, want %q", loc, want)
	}
}

func Test_pageLinks(t *testing.T) {
	tests := []struct {
		name          string
		current, last int
		want          string // gaps as "…", the current page in brackets
	}{
		{"single page", 1, 1, "[1]"},
		{"fits without gaps", 3, 5, "1 2 [3] 4 5"},
		{"first page", 1, 20, "[1] 2 3 … 20"},
		{"last page", 20, 20, "1 … 18 19 [20]"},
		{"middle", 10, 20, "1 … 8 9 [10] 11 12 … 20"},
		{"one hidden page is shown", 5, 20, "1 2 3 4 [5] 6 7 … 20"},
		{"two hidden pages become a gap", 6, 20, "1 … 4 5 [6] 7 8 … 20"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, l := range pageLinks("", tt.current, tt.last, 2) {
				switch {
				case l.Number == 0:
					got = append(got, "…")
				case l.Current:
					got = append(got, fmt.Sprintf("[%d]", l.Number))
				default:
					got = append(got, strconv.Itoa(l.Number))
				}
				if want := homeURL("", l.Number); l.Number != 0 && l.URL != want {
					t.Errorf("page %d URL = %q, want %q", l.Number, l.URL, want)
				}
			}
			if s := strings.Join(got, " "); s != tt.want {
				t.Errorf("pageLinks(%d, %d) = %q, want %q", tt.current, tt.last, s, tt.want)
			}
		})
	}
}

func Test_homeHandler_NoPageNavForOnePage(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(uuid.Must(uuid.NewV7()), "Only task", "Description"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	homeHandler(store, &fakePublisher{})(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if strings.Contains(rec.Body.String(), "<nav") {
		t.Errorf("rendered page navigation for a single page")
	}
}
