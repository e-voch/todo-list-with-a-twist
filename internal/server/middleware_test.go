package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveLogged runs one request through logRequests and returns the log line
// it wrote, decoded.
func serveLogged(t *testing.T, h http.Handler, target string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	logRequests(logger, h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log output %q is not one JSON line: %v", buf.String(), err)
	}
	return line
}

func Test_logRequests_StatusAndLevel(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus float64 // JSON numbers decode as float64
		wantLevel  string
		wantBytes  float64 // -1 skips the check
	}{
		{"writes nothing", func(w http.ResponseWriter, r *http.Request) {}, 200, "INFO", 0},
		{"body only", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) }, 200, "INFO", 5},
		{"redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/", http.StatusSeeOther) }, 303, "INFO", -1},
		{"not found", http.NotFound, 404, "WARN", 19},
		{"server error", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }, 500, "ERROR", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := serveLogged(t, tt.handler, "/")
			if line["msg"] != "request" {
				t.Errorf("msg = %v, want request", line["msg"])
			}
			if line["status"] != tt.wantStatus {
				t.Errorf("status = %v, want %v", line["status"], tt.wantStatus)
			}
			if line["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %v", line["level"], tt.wantLevel)
			}
			if tt.wantBytes >= 0 && line["bytes"] != tt.wantBytes {
				t.Errorf("bytes = %v, want %v", line["bytes"], tt.wantBytes)
			}
			if d, ok := line["duration_ms"].(float64); !ok || d < 0 {
				t.Errorf("duration_ms = %v, want a non-negative number", line["duration_ms"])
			}
		})
	}
}

func Test_logRequests_RouteAndPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/edit/", func(w http.ResponseWriter, r *http.Request) {})

	line := serveLogged(t, mux, "/edit/?id=123&ftitle=secret")

	if line["route"] != "/edit/" {
		t.Errorf("route = %v, want /edit/", line["route"])
	}
	if line["path"] != "/edit/" {
		t.Errorf("path = %v, want /edit/", line["path"])
	}
	for k, v := range line {
		if s, ok := v.(string); ok && strings.Contains(s, "secret") {
			t.Errorf("field %s = %q leaks the query string", k, s)
		}
	}
}

func Test_logRequests_URLTooLong(t *testing.T) {
	h := limitURLLength(http.NewServeMux())

	line := serveLogged(t, h, "/"+strings.Repeat("a", maxURLLength))

	if line["status"] != float64(http.StatusRequestURITooLong) {
		t.Errorf("status = %v, want %d", line["status"], http.StatusRequestURITooLong)
	}
	if line["route"] != "" {
		t.Errorf("route = %v, want empty: the request never reached the mux", line["route"])
	}
}
