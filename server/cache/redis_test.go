package cache

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisStoreGetSetDelete(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	prev := Default
	t.Cleanup(func() { Default = prev })

	if err := InitRedis(mr.Addr()); err != nil {
		t.Fatalf("InitRedis: %v", err)
	}

	ctx := context.Background()
	if err := Default.Set(ctx, "nearby:1", "payload", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := Default.Get(ctx, "nearby:1")
	if !ok || got != "payload" {
		t.Fatalf("Get: got %q ok=%v", got, ok)
	}
	if err := Default.Delete(ctx, "nearby:1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := Default.Get(ctx, "nearby:1"); ok {
		t.Fatal("expected key deleted")
	}
}
