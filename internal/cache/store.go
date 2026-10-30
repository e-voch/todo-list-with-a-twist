package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"todo/internal/task"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// timeout bounds each Redis call, so a slow Redis can't slow every request.
const timeout = 100 * time.Millisecond

// genKey holds a counter that is part of every cache key. Bumping it makes
// every existing entry unreachable at once; their TTL then removes them.
const genKey = "todo:gen"

// Store wraps a task.Store and caches List and Count in Redis. Create, Update
// and Delete invalidate the cache; every other method passes straight through
// to the wrapped store.
type Store struct {
	task.Store
	rdb *redis.Client
	ttl time.Duration
}

func New(inner task.Store, rdb *redis.Client, ttl time.Duration) *Store {
	return &Store{Store: inner, rdb: rdb, ttl: ttl}
}

func (s *Store) Create(id uuid.UUID, title, description string) error {
	if err := s.Store.Create(id, title, description); err != nil {
		return err // nothing changed, so nothing to invalidate
	}
	s.invalidate()
	return nil
}

func (s *Store) Update(id uuid.UUID, title, description string) error {
	if err := s.Store.Update(id, title, description); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

func (s *Store) Delete(id uuid.UUID) error {
	if err := s.Store.Delete(id); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

// generation returns the current counter. A missing counter is generation 0.
func (s *Store) generation(ctx context.Context) (int64, error) {
	gen, err := s.rdb.Get(ctx, genKey).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return gen, err
}

func (s *Store) invalidate() {
	Invalidate(s.rdb)
}

// Invalidate makes every cached entry stale. Code that writes tasks without
// going through Store, such as the worker, calls it after each write. If
// Redis is down this only logs: the write already succeeded, and the TTL
// clears stale entries soon anyway.
func Invalidate(rdb *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := rdb.Incr(ctx, genKey).Err(); err != nil {
		slog.Error("cache invalidate failed", "err", err)
	}
}

// List serves from Redis when it can. Any Redis failure falls back to the
// wrapped store, so a broken cache never fails a request.
func (s *Store) List(query string, limit, offset int) ([]task.Task, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	gen, err := s.generation(ctx)
	if err != nil {
		cancel()
		slog.Error("cache generation failed", "err", err)
		return s.Store.List(query, limit, offset) // can't trust the cache: skip it
	}
	// query goes last: it is user input and may itself contain ':'.
	key := fmt.Sprintf("todo:v1:gen%d:list:%d:%d:%s", gen, limit, offset, query)

	data, err := s.rdb.Get(ctx, key).Bytes()
	cancel()

	switch {
	case err == nil:
		var tasks []task.Task
		if err := json.Unmarshal(data, &tasks); err == nil {
			slog.Info("cache hit", "key", key)
			return tasks, nil
		}
		slog.Warn("cache entry unreadable", "key", key, "err", err)
	case errors.Is(err, redis.Nil):
		slog.Info("cache miss", "key", key)
	default:
		slog.Error("cache get failed", "key", key, "err", err)
	}

	tasks, err := s.Store.List(query, limit, offset)
	if err != nil {
		return nil, err // never cache errors
	}

	data, err = json.Marshal(tasks)
	if err != nil {
		slog.Error("cache encode failed", "key", key, "err", err)
		return tasks, nil
	}
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.rdb.Set(ctx, key, data, s.ttl).Err(); err != nil {
		slog.Error("cache set failed", "key", key, "err", err)
	}
	return tasks, nil
}

func (s *Store) Count(query string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	gen, err := s.generation(ctx)
	if err != nil {
		cancel()
		slog.Error("cache generation failed", "err", err)
		return s.Store.Count(query) // can't trust the cache: skip it
	}
	key := fmt.Sprintf("todo:v1:gen%d:count:%s", gen, query)

	data, err := s.rdb.Get(ctx, key).Bytes()
	cancel()

	switch {
	case err == nil:
		var n int
		if err := json.Unmarshal(data, &n); err == nil {
			slog.Info("cache hit", "key", key)
			return n, nil
		}
		slog.Warn("cache entry unreadable", "key", key, "err", err)
	case errors.Is(err, redis.Nil):
		slog.Info("cache miss", "key", key)
	default:
		slog.Error("cache get failed", "key", key, "err", err)
	}
	n, err := s.Store.Count(query) // ask Postgres
	if err != nil {
		return 0, err // don't save errors
	}

	data, err = json.Marshal(n) // int to JSON bytes
	if err != nil {
		slog.Error("cache encode failed", "key", key, "err", err)
		return n, nil
	}
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.rdb.Set(ctx, key, data, s.ttl).Err(); err != nil {
		slog.Error("cache set failed", "key", key, "err", err)
	}
	return n, nil
}
