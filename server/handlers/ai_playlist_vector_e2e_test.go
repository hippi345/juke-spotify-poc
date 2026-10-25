package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/vector"
)

type captureGen struct {
	candidates []gemini.TrackSuggestion
	picks      []gemini.TrackSuggestion
}

func (g *captureGen) GenerateTrackList(desc string, count int, similar []string, opts gemini.GenerateOptions) ([]gemini.TrackSuggestion, error) {
	g.candidates = opts.PreferredCandidates
	// Return planted candidates when injected (vector ON path).
	if len(opts.PreferredCandidates) >= count {
		g.picks = opts.PreferredCandidates[:count]
		return g.picks, nil
	}
	out := make([]gemini.TrackSuggestion, count)
	for i := range out {
		out[i] = gemini.TrackSuggestion{Title: "Fallback", Artist: "None"}
	}
	g.picks = out
	return out, nil
}

type fakeSpotify struct{}

func (f fakeSpotify) GetDefaultAccount() (*models.SpotifyAccount, error) {
	return &models.SpotifyAccount{ID: 1}, nil
}

func (f fakeSpotify) SearchTracks(q string, limit int) ([]spotify.Track, error) {
	return []spotify.Track{{
		ID:      "fake-id",
		Name:    q,
		URI:     "spotify:track:fake-id",
		Artists: []spotify.Artist{{Name: "Artist"}},
	}}, nil
}

func (f fakeSpotify) CreatePlaylist(name, desc string, _ bool) (string, error) {
	return "pl-fake", nil
}

func (f fakeSpotify) AddTracksToPlaylist(string, []string) error { return nil }

// TestAIPlaylistVectorE2E exercises OpenSearch-style retrieval → candidate injection → mocked Gemini → fake Spotify.
func TestAIPlaylistVectorE2E(t *testing.T) {
	path := vector.DefaultCatalogFixturePath()
	cat, err := vector.LoadVenueCatalogFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	roof := cat.VenueRooftopID
	if roof == 0 {
		roof = 2
	}
	idx := &e2eMemIndexer{docs: map[string]vector.VenueTrackDoc{}}
	emb := vector.HashEmbedder{Dim: vector.IndexEmbeddingDim}
	for _, tr := range cat.Tracks {
		if tr.VenueID != roof || !tr.PlantedSunset {
			continue
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, tr.Genres+" "+tr.Mood+" crowd favorite sunset")
		vec, err := emb.Embed(text)
		if err != nil {
			t.Fatal(err)
		}
		docID := "2-" + tr.SpotifyID
		idx.docs[docID] = vector.VenueTrackDoc{
			VenueID: roof, SpotifyID: tr.SpotifyID, Title: tr.Title, Artist: tr.Artist,
			Genres: tr.Genres, VoteCount: tr.VoteCount, Embedding: vec,
		}
	}
	vector.SetDefaultService(vector.Service{
		Enabled: true, Indexer: idx, Embedder: emb, RetrievalLimit: 20, MinSimilarityScore: 0.001,
	})

	cfg := &config.Config{
		GeminiAPIKey:           "fake-key-for-test",
		VibeSenseVectorEnabled: true,
	}
	gen := &captureGen{}
	planted := map[string]bool{}
	for _, tr := range cat.Tracks {
		if tr.PlantedSunset {
			planted[stringsLowerKey(tr.Title, tr.Artist)] = true
		}
	}

	jobID := "e2e-test"
	setJob(jobID, &aiPlaylistJob{Status: "pending"})
	runAIPlaylistJobWithDeps(jobID, cfg, gen, fakeSpotify{}, aiPlaylistRequest{
		PlaylistName: "SAMPLE sunset",
		TrackCount:   8,
		Description:  "play sunset songs based on what folks have liked in the past",
		VenueID:      roof,
	})
	j, ok := getJob(jobID)
	if !ok || j.Status != "completed" {
		t.Fatalf("job status %v err=%s", j.Status, j.ErrMsg)
	}
	if len(gen.candidates) == 0 {
		t.Fatal("expected injected candidates")
	}
	var fromPlanted int
	for _, p := range gen.picks {
		if planted[stringsLowerKey(p.Title, p.Artist)] {
			fromPlanted++
		}
	}
	if fromPlanted < 5 {
		t.Fatalf("expected picks from planted set, got %d/%d", fromPlanted, len(gen.picks))
	}
}

func stringsLowerKey(title, artist string) string {
	return strings.ToLower(strings.TrimSpace(title) + "|" + strings.TrimSpace(artist))
}

type e2eMemIndexer struct {
	docs map[string]vector.VenueTrackDoc
}

func (m *e2eMemIndexer) EnsureIndex(context.Context, int) error { return nil }
func (m *e2eMemIndexer) Get(context.Context, string) (*vector.VenueTrackDoc, error) { return nil, nil }
func (m *e2eMemIndexer) Upsert(_ context.Context, docID string, doc vector.VenueTrackDoc) error {
	m.docs[docID] = doc
	return nil
}
func (m *e2eMemIndexer) DeleteByVenueID(context.Context, uint) (int, error) { return 0, nil }
func (m *e2eMemIndexer) SearchSimilar(_ context.Context, venueID uint, queryVec []float32, limit int, _ time.Time) ([]vector.SearchHit, error) {
	var hits []vector.SearchHit
	for _, d := range m.docs {
		if d.VenueID != venueID {
			continue
		}
		hits = append(hits, vector.SearchHit{Title: d.Title, Artist: d.Artist, Score: 1})
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
func (m *e2eMemIndexer) SearchByText(ctx context.Context, venueID uint, q string, embedder vector.Embedder, limit int, after time.Time) ([]vector.SearchHit, error) {
	vec, err := embedder.Embed(q)
	if err != nil {
		return nil, err
	}
	return m.SearchSimilar(ctx, venueID, vec, limit, after)
}

type e2eSpotifyClient interface {
	GetDefaultAccount() (*models.SpotifyAccount, error)
	SearchTracks(string, int) ([]spotify.Track, error)
	CreatePlaylist(string, string, bool) (string, error)
	AddTracksToPlaylist(string, []string) error
}

// runAIPlaylistJobWithDeps is the production job path with injectable generator/client (tests only).
func runAIPlaylistJobWithDeps(id string, cfg *config.Config, gen gemini.PlaylistGenerator, client e2eSpotifyClient, body aiPlaylistRequest) {
	updateJob(id, func(j *aiPlaylistJob) { j.Status = "running" })
	var opts gemini.GenerateOptions
	if cfg.VibeSenseVectorEnabled && body.VenueID > 0 {
		opts.PreferredCandidates = vector.Default().CandidatesForMood(context.Background(), body.VenueID, body.Description, 0, time.Time{})
	}
	suggestions, err := gen.GenerateTrackList(body.Description, body.TrackCount, body.SimilarArtists, opts)
	if err != nil {
		failJobGemini(id, cfg, err)
		return
	}
	acc, err := client.GetDefaultAccount()
	if err != nil || acc == nil {
		failJob(id, "Spotify account disconnected")
		return
	}
	var uris []string
	for _, s := range suggestions {
		if len(uris) >= body.TrackCount {
			break
		}
		tracks, err := client.SearchTracks(s.Artist+" "+s.Title, 5)
		if err != nil || len(tracks) == 0 {
			continue
		}
		uris = append(uris, tracks[0].URI)
	}
	if len(uris) == 0 {
		failJob(id, "no tracks")
		return
	}
	plID, err := client.CreatePlaylist(body.PlaylistName, "e2e", false)
	if err != nil {
		failJob(id, err.Error())
		return
	}
	_ = client.AddTracksToPlaylist(plID, uris)
	completeJob(id, gin.H{"playlist_id": plID, "tracks_added": len(uris)})
}
