package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// httpMetrics are the request metrics measureRequests updates.
type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// newHTTPMetrics creates the request metrics and registers them with reg, so
// they appear on the /metrics page that serves reg.
func newHTTPMetrics(reg prometheus.Registerer) *httpMetrics {
	m := &httpMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests served, by method, route and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "How long HTTP requests took, by method and route.",
			Buckets: prometheus.DefBuckets, // 5ms up to 10s
		}, []string{"method", "route"}),
	}
	reg.MustRegister(m.requests, m.duration)
	return m
}

// measureRequests counts every request and records how long it took.
func measureRequests(next http.Handler, m *httpMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		// The route is the mux pattern that matched, like "/edit/", not the raw
		// path: raw paths are endless (every 404 is new), and each distinct
		// label value is a separate series Prometheus has to store.
		route := r.Pattern
		if route == "" {
			route = "unmatched" // rejected before reaching the mux, e.g. URL too long
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
