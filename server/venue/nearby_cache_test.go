package venue

import (
	"context"
	"testing"
	"time"

	"juke-spotify-poc/server/cache"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestInvalidateNearbyCaches_clearsPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	prev := cache.Default
	t.Cleanup(func() { cache.Default = prev })

	if err := cache.InitRedis(mr.Addr()); err != nil {
		t.Fatalf("InitRedis: %v", err)
	}

	ctx := context.Background()
	if err := cache.Default.Set(ctx, "jukespotify:nearby:37.77490:-122.41940:2000", "cached", 30*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := cache.Default.Set(ctx, "jukespotify:other:key", "keep", 30*time.Second); err != nil {
		t.Fatalf("set other: %v", err)
	}

	InvalidateNearbyCaches(ctx)

	if mr.Exists("jukespotify:nearby:37.77490:-122.41940:2000") {
		t.Fatal("expected nearby key removed")
	}
	if !mr.Exists("jukespotify:other:key") {
		t.Fatal("expected unrelated key to remain")
	}
}
