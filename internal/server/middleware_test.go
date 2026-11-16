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

func Test_logRequests(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    int
	}{
		{"implicit 200", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }, http.StatusOK},
		{"explicit status", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Log into a buffer instead of discarding, so the test can read the line back.
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			rec := httptest.NewRecorder()
			logRequests(tt.handler, logger).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/edit/?id=1", nil))

			if rec.Code != tt.want {
				t.Errorf("response status = %d, want %d", rec.Code, tt.want)
			}

			var line struct {
				Msg        string   `json:"msg"`
				Method     string   `json:"method"`
				Path       string   `json:"path"`
				Status     int      `json:"status"`
				DurationMS *float64 `json:"duration_ms"`
			}
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("want exactly one JSON log line, got %q: %v", buf.String(), err)
			}
			if line.Msg != "request" || line.Method != http.MethodPost || line.Path != "/edit/" || line.Status != tt.want {
				t.Errorf("log = %+v, want msg=request method=POST path=/edit/ status=%d", line, tt.want)
			}
			if line.DurationMS == nil {
				t.Error("log has no duration_ms")
			}
		})
	}
}

func Test_logRequests_SkipsMetrics(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	served := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served = true })

	logRequests(next, logger).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !served {
		t.Error("scrape was not passed on to the handler")
	}
	if buf.Len() != 0 {
		t.Errorf("scrape was logged: %q", buf.String())
	}
}

func Test_limitURLLength(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := limitURLLength(next, discard)

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
