package spotify

import "testing"

func TestSessionWalkFixture_playlistTracks(t *testing.T) {
	resp, err := getSessionWalkPlaylistTracks(0)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total < 4 {
		t.Fatalf("expected at least 4 fixture tracks, got %d", resp.Total)
	}
	if len(resp.Items) != resp.Total {
		t.Fatalf("expected %d items on first page, got %d", resp.Total, len(resp.Items))
	}
}

func TestSessionWalkFixture_currentlyPlaying(t *testing.T) {
	setSessionWalkNowPlaying("spotify:track:walk-track-bravo")
	cp, err := getSessionWalkCurrentlyPlaying()
	if err != nil || cp == nil || cp.Item == nil {
		t.Fatalf("currently playing: %v", err)
	}
	if cp.Item.ID != "walk-track-bravo" {
		t.Fatalf("expected bravo, got %s", cp.Item.ID)
	}
}
