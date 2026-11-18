// Package metrics holds the Prometheus setup that every program shares: the
// registry they start from and the handler that serves it.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRegistry returns a registry with the Go runtime and process metrics
// (memory, goroutines, GC, CPU, open files) already registered. A program
// creates one and hands it to every package that has metrics, like the logger.
func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

// NewHandler serves /metrics for Prometheus to scrape: every metric registered
// with reg, whichever package registered it. Run it on its own port, which
// the load balancer doesn't forward, so only the private network can read it.
func NewHandler(reg prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return mux
}
