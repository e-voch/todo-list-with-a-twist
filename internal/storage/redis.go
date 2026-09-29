package storage

import (
	"context"
	"fmt"
	"strconv"

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
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, err
		}

		t, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		results = append(results, t)
	}

	return results, nil
}

func (s *RedisStore) Get(id int) (task.Task, error) {
	ctx := context.Background()
	key := fmt.Sprintf("task:%d", id)

	fields, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return task.Task{}, err
	}
	if len(fields) == 0 {
		return task.Task{}, fmt.Errorf("task %d not found", id)
	}

	return task.Task{ID: id, Title: fields["title"], Description: fields["description"]}, nil
}

func (s *RedisStore) Create(title, description string) (id int, err error) {
	ctx := context.Background()

	newID, err := s.client.Incr(ctx, "task_id_counter").Result()
	if err != nil {
		return 0, err
	}

	key := fmt.Sprintf("task:%d", newID)
	if err := s.client.HSet(ctx, key, "title", title, "description", description).Err(); err != nil {
		return 0, err
	}

	if err := s.client.SAdd(ctx, "tasks", newID).Err(); err != nil {
		return 0, err
	}

	return int(newID), nil
}

func (s *RedisStore) Update(id int, title, description string) error {
	ctx := context.Background()
	key := fmt.Sprintf("task:%d", id)

	return s.client.HSet(ctx, key, "title", title, "description", description).Err()
}

func (s *RedisStore) Delete(id int) error {
	ctx := context.Background()
	key := fmt.Sprintf("task:%d", id)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return err
	}

	return s.client.SRem(ctx, "tasks", id).Err()
}
