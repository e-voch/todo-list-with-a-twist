package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func Test_NewHandler(t *testing.T) {
	// A metric registered by another package, as the cache registers its own:
	// /metrics must serve everything in the registry.
	reg := NewRegistry()
	reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "registered_elsewhere_total", Help: "test"}))
	rec := httptest.NewRecorder()

	NewHandler(reg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"registered_elsewhere_total",
		"go_goroutines",             // from the Go collector NewRegistry adds
		"process_cpu_seconds_total", // from the process collector
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics does not serve %s", want)
		}
	}
}
