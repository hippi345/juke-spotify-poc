package cache

import (
	"context"
	"time"
)

// Store is a simple key/value cache (Redis when configured).
type Store interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type nopStore struct{}

func (nopStore) Get(context.Context, string) (string, bool) { return "", false }
func (nopStore) Set(context.Context, string, string, time.Duration) error { return nil }
func (nopStore) Delete(context.Context, string) error { return nil }

// Default is a no-op until InitRedis succeeds.
var Default Store = nopStore{}
