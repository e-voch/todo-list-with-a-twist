package cache

import (
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"

	"todo/internal/task"
)

// innerStore answers List and Count; the cache under test only calls those.
type innerStore struct {
	task.Store
}

func (innerStore) List(string, int, int) ([]task.Task, error) {
	return []task.Task{{Title: "Buy milk"}}, nil
}

func (innerStore) Count(string) (int, error) { return 1, nil }

// With Redis unreachable, every lookup must fall back to the inner store and
// be counted as an error, not as a hit or a miss.
func TestStore_CountsErrorsWhenRedisIsDown(t *testing.T) {
	// Nothing listens on port 1, so every Redis call fails at once.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:1", MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	s := New(innerStore{}, rdb, 0, slog.New(slog.DiscardHandler), prometheus.NewRegistry())

	if tasks, err := s.List("", 10, 0); err != nil || len(tasks) != 1 {
		t.Fatalf("List = %v, %v; want the inner store's 1 task", tasks, err)
	}
	if n, err := s.Count(""); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v; want the inner store's 1", n, err)
	}

	for _, tt := range []struct{ op, result string }{
		{"list", "error"}, {"count", "error"},
	} {
		got := testutil.ToFloat64(s.requests.WithLabelValues(tt.op, tt.result))
		if got != 1 {
			t.Errorf("cache_requests_total{op=%q,result=%q} = %v, want 1", tt.op, tt.result, got)
		}
	}
	for _, op := range []string{"list", "count"} {
		for _, result := range []string{"hit", "miss"} {
			if got := testutil.ToFloat64(s.requests.WithLabelValues(op, result)); got != 0 {
				t.Errorf("cache_requests_total{op=%q,result=%q} = %v, want 0", op, result, got)
			}
		}
	}
}
