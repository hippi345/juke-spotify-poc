package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"gorm.io/gorm"
)

const (
	spotifyAuthURL  = "https://accounts.spotify.com"
	spotifyAPIURL   = "https://api.spotify.com/v1"
	tokenRefreshBuf = 5 * time.Minute
)

// Client wraps Spotify API calls with token management
type Client struct {
	cfg    *config.Config
	client *http.Client
}

// NewClient creates a Spotify API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetDefaultAccount returns the first connected Spotify account (POC: single account)
func (c *Client) GetDefaultAccount() (*models.SpotifyAccount, error) {
	var acc models.SpotifyAccount
	err := db.DB.Order("id ASC").First(&acc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}

// EnsureValidToken refreshes the access token if expired, updates DB
func (c *Client) EnsureValidToken(acc *models.SpotifyAccount) error {
	if time.Until(acc.TokenExpiresAt) > tokenRefreshBuf {
		return nil
	}

	tokenURL := spotifyAuthURL + "/api/token"
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", acc.RefreshToken)

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.cfg.SpotifyClientID, c.cfg.SpotifyClientSecret)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed: %s %s", resp.Status, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	acc.AccessToken = result.AccessToken
	acc.TokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return db.DB.Save(acc).Error
}

// Do performs an authenticated request to the Spotify API
func (c *Client) Do(method, path string, body io.Reader) (*http.Response, error) {
	acc, err := c.GetDefaultAccount()
	if err != nil || acc == nil {
		return nil, fmt.Errorf("no spotify account connected")
	}

	if err := c.EnsureValidToken(acc); err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}

	fullURL := spotifyAPIURL + path
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// Get performs a GET request
func (c *Client) Get(path string) (*http.Response, error) {
	return c.Do(http.MethodGet, path, nil)
}

// DoWithQuery performs a request with query parameters (for POST with query params)
func (c *Client) DoWithQuery(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	acc, err := c.GetDefaultAccount()
	if err != nil || acc == nil {
		return nil, fmt.Errorf("no spotify account connected")
	}

	if err := c.EnsureValidToken(acc); err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}

	fullURL := spotifyAPIURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// Post performs a POST request
func (c *Client) Post(path string, body io.Reader) (*http.Response, error) {
	return c.Do(http.MethodPost, path, body)
}

// GetCurrentlyPlaying fetches the currently playing track from Spotify
func (c *Client) GetCurrentlyPlaying() (*CurrentlyPlaying, error) {
	resp, err := c.Get("/me/player/currently-playing")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("currently-playing: %s %s", resp.Status, string(body))
	}

	var cp CurrentlyPlaying
	if err := json.NewDecoder(resp.Body).Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

// GetPlaylists fetches the user's playlists (only those they own, to avoid 403 on tracks)
func (c *Client) GetPlaylists() ([]PlaylistItem, error) {
	acc, err := c.GetDefaultAccount()
	if err != nil || acc == nil {
		return nil, fmt.Errorf("no spotify account connected")
	}

	var all []PlaylistItem
	path := "/me/playlists?limit=50"

	for path != "" {
		resp, err := c.Get(path)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("playlists: %s %s", resp.Status, string(body))
		}

		var pr PlaylistsResponse
		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, item := range pr.Items {
			if item.Owner.ID == acc.SpotifyUserID {
				all = append(all, item)
			}
		}

		if pr.Next != nil && *pr.Next != "" {
			u, err := url.Parse(*pr.Next)
			if err != nil {
				break
			}
			// Strip /v1 - spotifyAPIURL already includes it
			path = strings.TrimPrefix(u.Path, "/v1")
			if path == "" {
				path = "/"
			}
			if u.RawQuery != "" {
				path += "?" + u.RawQuery
			}
		} else {
			path = ""
		}
	}

	return all, nil
}

// GetPlaylistTracks fetches tracks from a playlist with pagination
func (c *Client) GetPlaylistTracks(playlistID string, offset int) (*PlaylistTracksResponse, error) {
	path := fmt.Sprintf("/playlists/%s/tracks?limit=50&offset=%d", playlistID, offset)
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("playlist tracks: %s %s", resp.Status, string(body))
	}

	var ptr PlaylistTracksResponse
	if err := json.NewDecoder(resp.Body).Decode(&ptr); err != nil {
		return nil, err
	}
	return &ptr, nil
}

// AddToQueue adds a track to the user's playback queue
func (c *Client) AddToQueue(trackURI string) error {
	q := url.Values{}
	q.Set("uri", trackURI)
	resp, err := c.DoWithQuery(http.MethodPost, "/me/player/queue", q, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add to queue: %s %s", resp.Status, string(body))
	}
	return nil
}

// GetRecommendations fetches track recommendations based on seed tracks
func (c *Client) GetRecommendations(seedTrackIDs []string, limit int) ([]Track, error) {
	if len(seedTrackIDs) == 0 {
		return nil, fmt.Errorf("at least one seed track required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Use up to 5 seeds (Spotify limit)
	seeds := seedTrackIDs
	if len(seeds) > 5 {
		seeds = seeds[:5]
	}

	q := url.Values{}
	q.Set("seed_tracks", strings.Join(seeds, ","))
	q.Set("limit", fmt.Sprintf("%d", limit))

	path := "/recommendations?" + q.Encode()
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recommendations: %s %s", resp.Status, string(body))
	}

	var rr RecommendationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, err
	}
	return rr.Tracks, nil
}

// AddTracksToPlaylist adds tracks to a playlist
func (c *Client) AddTracksToPlaylist(playlistID string, trackURIs []string) error {
	if len(trackURIs) == 0 {
		return nil
	}

	// Spotify allows up to 100 per request
	body := map[string]interface{}{
		"uris": trackURIs,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/playlists/%s/tracks", playlistID)
	resp, err := c.Post(path, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add tracks to playlist: %s %s", resp.Status, string(bodyBytes))
	}
	return nil
}
