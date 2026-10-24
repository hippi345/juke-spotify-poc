package venue

import (
	"context"
	"log"

	"juke-spotify-poc/server/cache"
)

const nearbyCachePrefix = "jukespotify:nearby:"

// InvalidateNearbyCaches clears cached nearby session lists (call on session start/end).
func InvalidateNearbyCaches(ctx context.Context) {
	if err := cache.DeleteByPrefix(ctx, nearbyCachePrefix); err != nil {
		log.Printf("venue: invalidate nearby cache: %v", err)
	}
}
