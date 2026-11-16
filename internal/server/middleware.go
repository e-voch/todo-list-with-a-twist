package server

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps a ResponseWriter and remembers the status code the
// handler sets. Every other method passes straight through to the real writer.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests writes one log line per request, after the handler has finished.
func logRequests(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// A handler that never calls WriteHeader gets 200 from Go, so start there.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
	})
}

// limitURLLength rejects requests whose URL is longer than maxURLLength.
func limitURLLength(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.RequestURI) > maxURLLength {
			logger.Warn("url too long", "length", len(r.RequestURI))
			http.Error(w, "URL too long", http.StatusRequestURITooLong)
			return
		}
		next.ServeHTTP(w, r)
	})
}
