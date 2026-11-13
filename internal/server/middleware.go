package server

import (
	"log/slog"
	"net/http"
	"time"
)

// logRequests writes one log line per request, after it has been served, with
// the response status and how long it took. Those two fields are what a
// dashboard needs for error rates and latency.
//
// The level follows the status: 5xx is an error, 4xx a warning, anything else
// info, so filtering on level:ERROR finds the requests that failed.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK // the handler wrote nothing, which net/http sends as 200
		}
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		logger.LogAttrs(r.Context(), level, "request",
			slog.String("method", r.Method),
			// The path only, not the query: a create puts the task's title and
			// description in the query string.
			slog.String("path", r.URL.Path),
			// The mux pattern that matched, like "/edit/". Unlike path it has
			// a handful of values, so it groups well in a chart. Empty when
			// the request never reached the mux, as with a URL that's too long.
			slog.String("route", r.Pattern),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
			slog.Int("bytes", rec.bytes),
		)
	})
}

// statusRecorder remembers the status code and body size a handler writes.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real ResponseWriter, for
// things like Flush.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
