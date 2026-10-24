package cache

import (
	"context"
)

// DeleteByPrefix removes all keys matching prefix* (Redis SCAN). No-op when cache is disabled.
func DeleteByPrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return nil
	}
	switch s := Default.(type) {
	case *redisStore:
		var cursor uint64
		for {
			keys, next, err := s.client.Scan(ctx, cursor, prefix+"*", 128).Result()
			if err != nil {
				return err
			}
			if len(keys) > 0 {
				if err := s.client.Del(ctx, keys...).Err(); err != nil {
					return err
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		return nil
	default:
		return nil
	}
}
