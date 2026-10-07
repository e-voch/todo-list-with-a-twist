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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), title) {
		t.Errorf("home page does not contain the new task %q", title)
	}
	return string(body)
}

func findTaskID(t *testing.T, body, title string) string {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(title) + `[^<]*<a href="/edit/\?id=([0-9a-f-]+)"`)
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

	defer resp.Body.Close()

	after, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(after), title) {
		t.Errorf("task %q is still on the home page after delete", title)
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

	defer resp.Body.Close()

	after, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(after), newTitle) {
		t.Errorf("edited title %q not on the home page", newTitle)
	}

	if strings.Contains(string(after), title) {
		t.Errorf("old title %q is still on the home page after edit", title)
	}
}
