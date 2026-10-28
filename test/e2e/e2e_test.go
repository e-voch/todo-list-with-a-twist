package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

func TestHomePage(t *testing.T) {
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("is the app running? (make up && make run): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func createTask(t *testing.T, title string) string {
	t.Helper()
	q := url.Values{"ftitle": {title}, "fdescription": {"some description"}}
	resp, err := http.Get(baseURL + "/?" + q.Encode())

	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := searchPage(t, title)
	if !listed(body, title) {
		t.Errorf("searching for the new task %q does not list it", title)
	}
	return body
}

// searchPage returns the home page filtered by query. Tests search for their
// task's unique title, so it is found however many other tasks there are.
func searchPage(t *testing.T, query string) string {
	t.Helper()
	resp, err := http.Get(baseURL + "/?" + url.Values{"q": {query}}.Encode())
	if err != nil {
		t.Fatalf("search for %q: %v", query, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("search status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// listed reports whether body lists a task with this title. A plain
// strings.Contains is not enough: a search page also shows the query itself.
func listed(body, title string) bool {
	return strings.Contains(body, "<span>"+title+" : ")
}

func findTaskID(t *testing.T, body, title string) string {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(title) + `[^<]*</span>\s*<a href="/edit/\?id=([0-9a-f-]+)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("task %q not found on the home page", title)
	}
	return m[1]
}

func TestHomePageCreate(t *testing.T) {
	title := fmt.Sprintf("delete-test-%d", time.Now().UnixNano())

	createTask(t, title)
}

func TestNotFound(t *testing.T) {
	resp, err := http.Get(baseURL + "/nope")
	if err != nil {
		t.Fatalf("is the app running? (make up && make run): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestDeleteTask(t *testing.T) {
	title := fmt.Sprintf("delete-test-%d", time.Now().UnixNano())
	body := createTask(t, title)
	id := findTaskID(t, body, title)

	resp, err := http.Get(baseURL + "/delete/?id=" + id)
	if err != nil {
		t.Fatalf("delete task: %v", err)
	}
	resp.Body.Close()

	if listed(searchPage(t, title), title) {
		t.Errorf("task %q is still listed after delete", title)
	}
}

func TestEditTask(t *testing.T) {
	title := fmt.Sprintf("Edit-test-%d", time.Now().UnixNano())
	body := createTask(t, title)
	id := findTaskID(t, body, title)

	newTitle := fmt.Sprintf("edited-%d", time.Now().UnixNano())
	q := url.Values{"id": {id}, "ftitle": {newTitle}, "fdescription": {"edited description"}}
	resp, err := http.Get(baseURL + "/edit/?" + q.Encode())
	if err != nil {
		t.Fatalf("edit task: %v", err)
	}
	resp.Body.Close()

	if !listed(searchPage(t, newTitle), newTitle) {
		t.Errorf("edited title %q is not listed", newTitle)
	}
	if listed(searchPage(t, title), title) {
		t.Errorf("old title %q is still listed after edit", title)
	}
}

// TestRecommendedSteps checks the whole pipeline: the server publishes a
// "created" event, the worker consumes it and stores steps, the page shows them.
func TestRecommendedSteps(t *testing.T) {
	title := fmt.Sprintf("steps-test-%d", time.Now().UnixNano())
	body := createTask(t, title)
	id := findTaskID(t, body, title)

	// "steps pending" is the placeholder; a bare "steps" class means the worker filled it in.
	done := regexp.MustCompile(regexp.QuoteMeta(id) + `">Delete</a>\s*<p class="steps">`)

	// Longer than Kafka's default 45s session timeout: if a previous worker died
	// without leaving the group, the new one isn't assigned the partition until then.
	const timeout = 60 * time.Second
	deadline := time.Now().Add(timeout)
	for {
		// Tasks are listed oldest first, so the new one is on the last page;
		// asking for a page past the end redirects there.
		resp, err := http.Get(baseURL + "/?page=1000000")
		if err != nil {
			t.Fatalf("get home page: %v", err)
		}
		page, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if done.Match(page) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no recommended steps for task %s after %s; is the worker running?", id, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
