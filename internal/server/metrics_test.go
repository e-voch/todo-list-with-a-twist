package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func Test_measureRequests(t *testing.T) {
	h := NewHandler(newFakeStore(), &fakePublisher{}, discard, prometheus.NewRegistry())

	for _, url := range []string{
		"/",
		"/no-such-page",
		"/another-missing-page",
		"/" + strings.Repeat("a", maxURLLength), // rejected by limitURLLength
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, url, nil))
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`http_requests_total{method="GET",route="/{$}",status="200"} 1`,
		// Two different missing paths share one series: the catch-all route.
		`http_requests_total{method="GET",route="/",status="404"} 2`,
		`http_requests_total{method="GET",route="unmatched",status="414"} 1`,
		`http_request_duration_seconds_count{method="GET",route="/{$}"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %s", want)
		}
	}
	if strings.Contains(body, "no-such-page") {
		t.Error("raw path leaked into a label")
	}
}
