package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
// Uses /items endpoint (the /tracks endpoint is deprecated and returns 403)
func (c *Client) GetPlaylistTracks(playlistID string, offset int) (*PlaylistTracksResponse, error) {
	path := fmt.Sprintf("/playlists/%s/items?limit=50&offset=%d", playlistID, offset)
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

// GetPlayerDevices fetches the user's available Spotify Connect devices
func (c *Client) GetPlayerDevices() ([]Device, error) {
	resp, err := c.Get("/me/player/devices")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("devices: %s %s", resp.Status, string(body))
	}
	var dr DevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, err
	}
	return dr.Devices, nil
}

// TransferPlayback transfers playback to a device (required before play when no active device)
func (c *Client) TransferPlayback(deviceID string, play bool) error {
	body, _ := json.Marshal(map[string]interface{}{
		"device_ids": []string{deviceID},
		"play":       play,
	})
	resp, err := c.Do(http.MethodPut, "/me/player", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("transfer playback: %s %s", resp.Status, string(respBody))
	}
	return nil
}

// StartPlayback starts playback with the given track URI (or resumes if uri is empty).
// Uses the account's ActiveDeviceID if set; otherwise falls back to auto-pick on 404.
func (c *Client) StartPlayback(trackURI string) error {
	acc, _ := c.GetDefaultAccount()
	preferredDevice := ""
	if acc != nil && acc.ActiveDeviceID != "" {
		preferredDevice = acc.ActiveDeviceID
		// Transfer to the preferred device first so it's active before play
		_ = c.TransferPlayback(preferredDevice, false)
	}

	err := c.startPlaybackWithDevice(trackURI, preferredDevice)
	if err == nil {
		return nil
	}
	// 404 often means "no active device" - transfer to preferred or first available, then retry
	if strings.Contains(err.Error(), "404") {
		devices, devErr := c.GetPlayerDevices()
		if devErr != nil || len(devices) == 0 {
			return err
		}
		// Try preferred device first if it's in the list
		if preferredDevice != "" {
			for _, d := range devices {
				if d.ID == preferredDevice && !d.IsRestricted {
					_ = c.TransferPlayback(d.ID, false)
					if retryErr := c.startPlaybackWithDevice(trackURI, d.ID); retryErr == nil {
						return nil
					}
					break
				}
			}
		}
		for _, d := range devices {
			if d.IsRestricted || d.ID == "" {
				continue
			}
			_ = c.TransferPlayback(d.ID, false)
			if retryErr := c.startPlaybackWithDevice(trackURI, d.ID); retryErr == nil {
				return nil
			}
		}
	}
	return err
}

func (c *Client) startPlaybackWithDevice(trackURI, deviceID string) error {
	var body io.Reader
	if trackURI != "" {
		jsonBody, _ := json.Marshal(map[string]interface{}{"uris": []string{trackURI}})
		body = bytes.NewReader(jsonBody)
	}
	var resp *http.Response
	var err error
	if deviceID != "" {
		q := url.Values{}
		q.Set("device_id", deviceID)
		resp, err = c.DoWithQuery(http.MethodPut, "/me/player/play", q, body)
	} else {
		resp, err = c.Do(http.MethodPut, "/me/player/play", body)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("start playback: %s %s", resp.Status, string(respBody))
	}
	return nil
}

// PausePlayback pauses playback on the user's device
func (c *Client) PausePlayback() error {
	acc, _ := c.GetDefaultAccount()
	var resp *http.Response
	var err error
	if acc != nil && acc.ActiveDeviceID != "" {
		q := url.Values{}
		q.Set("device_id", acc.ActiveDeviceID)
		resp, err = c.DoWithQuery(http.MethodPut, "/me/player/pause", q, nil)
	} else {
		resp, err = c.Do(http.MethodPut, "/me/player/pause", nil)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pause playback: %s %s", resp.Status, string(respBody))
	}
	return nil
}

// GetQueue fetches the user's playback queue (currently playing + queue items)
func (c *Client) GetQueue() (*QueueResponse, error) {
	resp, err := c.Get("/me/player/queue")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get queue: %s %s", resp.Status, string(body))
	}
	var qr QueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, err
	}
	return &qr, nil
}

// SkipToNext skips to the next track in the queue
func (c *Client) SkipToNext() error {
	acc, _ := c.GetDefaultAccount()
	var resp *http.Response
	var err error
	if acc != nil && acc.ActiveDeviceID != "" {
		q := url.Values{}
		q.Set("device_id", acc.ActiveDeviceID)
		resp, err = c.DoWithQuery(http.MethodPost, "/me/player/next", q, nil)
	} else {
		resp, err = c.Do(http.MethodPost, "/me/player/next", nil)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("skip to next: %s %s", resp.Status, string(respBody))
	}
	return nil
}

// ClearQueue clears the playback queue by skipping through all items. Spotify has no
// "clear queue" API, so we skip to next repeatedly and pause after each skip to avoid
// playing through the queue. Best-effort; errors are logged but not returned.
func (c *Client) ClearQueue() {
	qr, err := c.GetQueue()
	if err != nil {
		log.Printf("voting: clear queue (get queue): %v", err)
		return
	}
	count := len(qr.Queue)
	if count == 0 {
		return
	}
	for i := 0; i < count; i++ {
		if err := c.SkipToNext(); err != nil {
			log.Printf("voting: clear queue (skip): %v", err)
			return
		}
		_ = c.PausePlayback()
		time.Sleep(200 * time.Millisecond) // brief pause to let skip complete
	}
}

// AddToQueue adds a track to the user's playback queue
func (c *Client) AddToQueue(trackURI string) error {
	acc, _ := c.GetDefaultAccount()
	q := url.Values{}
	q.Set("uri", trackURI)
	if acc != nil && acc.ActiveDeviceID != "" {
		q.Set("device_id", acc.ActiveDeviceID)
	}
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
