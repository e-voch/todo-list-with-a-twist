// Package logging sets up the slog default logger the same way for every
// binary, configured from the environment:
//
//	LOG_LEVEL   debug, info (default), warn or error
//	LOG_FORMAT  json (default) or text
//
// JSON is what Fluent Bit parses into OpenSearch fields. Text is easier to read
// in a terminal during local development.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup makes a logger tagged with service the slog default. An unknown
// LOG_LEVEL or LOG_FORMAT falls back to the default and is reported with a
// warning, rather than stopping the service over a typo.
func Setup(service string) {
	logger, warnings := New(os.Stdout, service, os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"))
	slog.SetDefault(logger)
	for _, w := range warnings {
		slog.Warn(w)
	}
}

// New builds the logger Setup installs. It is separate so tests can write to a
// buffer and pass the settings in.
func New(w io.Writer, service, level, format string) (*slog.Logger, []string) {
	var warnings []string

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		if level != "" {
			warnings = append(warnings, fmt.Sprintf("unknown LOG_LEVEL %q, using info", level))
		}
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	switch strings.ToLower(format) {
	case "", "json":
		h = slog.NewJSONHandler(w, opts)
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		warnings = append(warnings, fmt.Sprintf("unknown LOG_FORMAT %q, using json", format))
		h = slog.NewJSONHandler(w, opts)
	}

	return slog.New(h).With("service", service), warnings
}
