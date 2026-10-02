package cache

import (
	"context"
	"testing"
	"time"
)

func TestNopStore(t *testing.T) {
	ctx := context.Background()
	var s Store = nopStore{}

	if got, ok := s.Get(ctx, "k"); ok || got != "" {
		t.Fatalf("Get: got %q ok=%v, want miss", got, ok)
	}
	if err := s.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, ok := s.Get(ctx, "k"); ok || got != "" {
		t.Fatalf("Get after Set on nop: got %q ok=%v", got, ok)
	}
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestInitRedisEmptyAddr(t *testing.T) {
	prev := Default
	t.Cleanup(func() { Default = prev })

	if err := InitRedis(""); err != nil {
		t.Fatalf("InitRedis empty: %v", err)
	}
	var _ Store = Default
}
