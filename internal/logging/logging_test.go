package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_Level(t *testing.T) {
	tests := []struct {
		level        string
		debug, info  bool
		wantWarnings int
	}{
		{"", false, true, 0},
		{"info", false, true, 0},
		{"debug", true, true, 0},
		{"DEBUG", true, true, 0},
		{"warn", false, false, 0},
		{"verbose", false, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			logger, warnings := New(&buf, "test", tt.level, "")

			logger.Debug("d")
			gotDebug := buf.Len() > 0
			buf.Reset()
			logger.Info("i")
			gotInfo := buf.Len() > 0

			if gotDebug != tt.debug || gotInfo != tt.info {
				t.Errorf("debug logged = %v, info logged = %v; want %v, %v", gotDebug, gotInfo, tt.debug, tt.info)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("warnings = %q, want %d", warnings, tt.wantWarnings)
			}
		})
	}
}

func TestNew_Format(t *testing.T) {
	for _, format := range []string{"", "json", "JSON", "xml"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			logger, warnings := New(&buf, "todo", "", format)
			logger.Info("hello", "task_id", "abc")

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("format %q: output %q is not JSON: %v", format, buf.String(), err)
			}
			if line["service"] != "todo" || line["task_id"] != "abc" || line["msg"] != "hello" {
				t.Errorf("line = %v, want service, task_id and msg set", line)
			}
			if wantWarn := format == "xml"; (len(warnings) > 0) != wantWarn {
				t.Errorf("warnings = %q, want warning: %v", warnings, wantWarn)
			}
		})
	}

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		logger, _ := New(&buf, "todo", "", "text")
		logger.Info("hello")
		if out := buf.String(); !strings.Contains(out, "msg=hello") || !strings.Contains(out, "service=todo") {
			t.Errorf("text output = %q, want key=value pairs", out)
		}
	})
}
