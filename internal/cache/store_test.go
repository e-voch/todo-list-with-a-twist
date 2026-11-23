package cache

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// slowStore counts its calls and holds each one until release is closed, so
// a test can line up many requests while the first is still running.
type slowStore struct {
	task.Store
	lists, counts atomic.Int32
	release       chan struct{}
}

func (s *slowStore) List(string, int, int) ([]task.Task, error) {
	s.lists.Add(1)
	<-s.release
	return []task.Task{{Title: "Buy milk"}}, nil
}

func (s *slowStore) Count(string) (int, error) {
	s.counts.Add(1)
	<-s.release
	return 1, nil
}

// Many identical lookups at the same moment must reach the database once, not
// once each: that is what stops a stampede after the cache is cleared.
func TestStore_MergesIdenticalLoads(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:1", MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	inner := &slowStore{release: make(chan struct{})}
	s := New(inner, rdb, 0, slog.New(slog.DiscardHandler), prometheus.NewRegistry())

	const callers = 10
	var wg sync.WaitGroup
	errs := make(chan error, 2*callers)
	for range callers {
		wg.Go(func() {
			tasks, err := s.List("task", 10, 0)
			if err == nil && len(tasks) != 1 {
				t.Errorf("List returned %d tasks, want 1", len(tasks))
			}
			errs <- err
		})
		wg.Go(func() {
			n, err := s.Count("task")
			if err == nil && n != 1 {
				t.Errorf("Count = %d, want 1", n)
			}
			errs <- err
		})
	}

	// Give every caller time to join the load already in flight, then let it finish.
	time.Sleep(200 * time.Millisecond)
	close(inner.release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
	}
	if got := inner.lists.Load(); got != 1 {
		t.Errorf("database List calls = %d, want 1", got)
	}
	if got := inner.counts.Load(); got != 1 {
		t.Errorf("database Count calls = %d, want 1", got)
	}
}
