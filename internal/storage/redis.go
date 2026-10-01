package storage

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"todo/internal/task"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})

	// NewClient connects lazily, so Ping to fail now if Redis is unreachable.
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &RedisStore{client: client}, nil
}

func (s *RedisStore) List() ([]task.Task, error) {
	ctx := context.Background()

	ids, err := s.client.SMembers(ctx, "tasks").Result()
	if err != nil {
		return nil, err
	}

	results := []task.Task{}
	for _, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, err
		}

		t, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		results = append(results, t)
	}

	// Set members come back in no particular order; UUIDv7s sort by creation time.
	slices.SortFunc(results, func(a, b task.Task) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return results, nil
}

func (s *RedisStore) Get(id uuid.UUID) (task.Task, error) {
	ctx := context.Background()
	key := taskKey(id)

	fields, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return task.Task{}, err
	}
	if len(fields) == 0 {
		return task.Task{}, fmt.Errorf("task %s not found", id)
	}

	return task.Task{ID: id, Title: fields["title"], Description: fields["description"]}, nil
}

func (s *RedisStore) Create(id uuid.UUID, title, description string) error {
	ctx := context.Background()
	key := taskKey(id)

	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 1 {
		return nil
	}

	if err := s.client.HSet(ctx, key, "title", title, "description", description).Err(); err != nil {
		return err
	}

	return s.client.SAdd(ctx, "tasks", id.String()).Err()
}

func (s *RedisStore) Update(id uuid.UUID, title, description string) error {
	ctx := context.Background()
	key := taskKey(id)

	return s.client.HSet(ctx, key, "title", title, "description", description).Err()
}

func (s *RedisStore) Delete(id uuid.UUID) error {
	ctx := context.Background()
	key := taskKey(id)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return err
	}

	return s.client.SRem(ctx, "tasks", id.String()).Err()
}

func taskKey(id uuid.UUID) string {
	return "task:" + id.String()
}
