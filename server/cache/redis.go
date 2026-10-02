package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

func (r *redisStore) Get(ctx context.Context, key string) (string, bool) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return val, true
}

func (r *redisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisStore) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// InitRedis connects to Redis at addr (host:port). Empty addr leaves the no-op store.
func InitRedis(addr string) error {
	if addr == "" {
		return nil
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return err
	}
	Default = &redisStore{client: client}
	return nil
}
