package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"gorm.io/gorm"
)

const refillConcurrency = 5 // Parallel artist fetches (balance speed vs rate limits)

const (
	spotifyAuthURL       = "https://accounts.spotify.com"
	spotifyAPIURL        = "https://api.spotify.com/v1"
	tokenRefreshBuf      = 5 * time.Minute
	playlistsCacheTTL    = 60 * time.Second
	refillCallDelayMs    = 100 // Delay between sequential calls (parallel artist fetches reduce total time)
)

var (
	playlistsCache   = make(map[string]playlistsCacheEntry)
	playlistsCacheMu sync.RWMutex
)

type playlistsCacheEntry struct {
	items []PlaylistItem
	at    time.Time
}

func getPlaylistsCache(userID string) []PlaylistItem {
	playlistsCacheMu.RLock()
	defer playlistsCacheMu.RUnlock()
	e, ok := playlistsCache[userID]
	if !ok || time.Since(e.at) > playlistsCacheTTL {
		return nil
	}
	return e.items
}

// getPlaylistsCacheStale returns cached playlists even if expired (for 429 fallback)
func getPlaylistsCacheStale(userID string) []PlaylistItem {
	playlistsCacheMu.RLock()
	defer playlistsCacheMu.RUnlock()
	e, ok := playlistsCache[userID]
	if !ok {
		return nil
	}
	return e.items
}

func setPlaylistsCache(userID string, items []PlaylistItem) {
	playlistsCacheMu.Lock()
	defer playlistsCacheMu.Unlock()
	playlistsCache[userID] = playlistsCacheEntry{items: items, at: time.Now()}
}

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

// Do performs an authenticated request to the Spotify API.
// Retries on 429 (rate limit) respecting Retry-After, up to 3 times.
func (c *Client) Do(method, path string, body io.Reader) (*http.Response, error) {
	return c.DoWithRetry(method, path, body, 3)
}

// Get performs a GET request
func (c *Client) Get(path string) (*http.Response, error) {
	return c.DoWithRetry(http.MethodGet, path, nil, 3)
}

// GetNoRetry performs a GET request without 429 retry (for endpoints with strict limits, e.g. playlists)
func (c *Client) GetNoRetry(path string) (*http.Response, error) {
	return c.DoWithRetry(http.MethodGet, path, nil, 0)
}

// DoWithRetry performs Do with configurable retry count. maxRetries=0 means no retry on 429.
func (c *Client) DoWithRetry(method, path string, body io.Reader, maxRetries int) (*http.Response, error) {
	acc, err := c.GetDefaultAccount()
	if err != nil || acc == nil {
		return nil, fmt.Errorf("no spotify account connected")
	}

	if err := c.EnsureValidToken(acc); err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}

	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}

	fullURL := spotifyAPIURL + path
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reqBody io.Reader
		if len(bodyBytes) > 0 {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequest(method, fullURL, reqBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
		if len(bodyBytes) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 429 {
			return resp, nil
		}

		// Log 429 details for debugging (Spotify may include error info in body)
		body429, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("spotify: 429 on %s %s - Retry-After: %s - body: %s", method, fullURL, resp.Header.Get("Retry-After"), string(body429))

		retryAfter := 5
		if v := resp.Header.Get("Retry-After"); v != "" {
			if n, parseErr := strconv.Atoi(v); parseErr == nil && n > 0 {
				if n > 15 {
					n = 15
				}
				retryAfter = n
			}
		}
		if attempt < maxRetries {
			log.Printf("spotify: 429 rate limit, waiting %ds before retry (attempt %d/%d)", retryAfter, attempt+1, maxRetries)
			time.Sleep(time.Duration(retryAfter) * time.Second)
		} else {
			return nil, fmt.Errorf("rate limited (429) after %d retries", maxRetries)
		}
	}
	return nil, fmt.Errorf("rate limited (429)")
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

// GetPlaylists fetches the user's playlists (only those they own, to avoid 403 on tracks).
// Uses cache (60s) and small delays between pages to reduce rate-limit pressure.
func (c *Client) GetPlaylists() ([]PlaylistItem, error) {
	acc, err := c.GetDefaultAccount()
	if err != nil || acc == nil {
		return nil, fmt.Errorf("no spotify account connected")
	}

	// Return cached playlists if fresh (avoids 429 on retry or quick re-open)
	if cached := getPlaylistsCache(acc.SpotifyUserID); cached != nil {
		return cached, nil
	}

	var all []PlaylistItem
	path := "/me/playlists?limit=50"
	firstPage := true

	for path != "" {
		if !firstPage {
			time.Sleep(150 * time.Millisecond) // Avoid bursting multiple requests
		}
		firstPage = false

		// Use same Get as devices - identical code path for debugging
		resp, err := c.Get(path)
		if err != nil {
			// On 429, return stale cache if available so user can at least select a playlist
			if strings.Contains(err.Error(), "429") {
				if stale := getPlaylistsCacheStale(acc.SpotifyUserID); len(stale) > 0 {
					log.Printf("spotify: playlists 429, returning %d cached items (may be stale)", len(stale))
					return stale, nil
				}
				return nil, fmt.Errorf("Spotify rate limited - wait 30 seconds and try again")
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 429 {
				if stale := getPlaylistsCacheStale(acc.SpotifyUserID); len(stale) > 0 {
					log.Printf("spotify: playlists 429, returning %d cached items (may be stale)", len(stale))
					return stale, nil
				}
				return nil, fmt.Errorf("Spotify rate limited - wait 30 seconds and try again")
			}
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

	setPlaylistsCache(acc.SpotifyUserID, all)
	return all, nil
}

// GetPlaylistTracks fetches tracks from a playlist with pagination
// Uses /items endpoint (the /tracks endpoint is deprecated and returns 403)
// Explicitly requests album.images so all tracks (including refill) display artwork
func (c *Client) GetPlaylistTracks(playlistID string, offset int) (*PlaylistTracksResponse, error) {
	fields := "items(track(id,name,uri,artists,album(id,name,images),duration_ms),item(id,name,uri,artists,album(id,name,images),duration_ms)),next,total"
	path := fmt.Sprintf("/playlists/%s/items?limit=50&offset=%d&fields=%s", playlistID, offset, url.QueryEscape(fields))
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
// "clear queue" API, so we skip to next repeatedly. Pause before/after each skip to
// avoid brief audio bursts. Best-effort; errors are logged but not returned.
func (c *Client) ClearQueue() {
	_ = c.PausePlayback()
	time.Sleep(100 * time.Millisecond)

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
		_ = c.PausePlayback() // ensure paused before skip to minimize audio
		if err := c.SkipToNext(); err != nil {
			log.Printf("voting: clear queue (skip): %v", err)
			return
		}
		_ = c.PausePlayback() // pause immediately after skip (skip can start next track)
		time.Sleep(80 * time.Millisecond)
	}
}

// AddToQueue adds a track to the user's playback queue.
// On 404 (no active device), transfers to preferred or first available device and retries.
// Does NOT call TransferPlayback on the happy path—that can pause playback.
func (c *Client) AddToQueue(trackURI string) error {
	acc, _ := c.GetDefaultAccount()
	preferredDevice := ""
	if acc != nil && acc.ActiveDeviceID != "" {
		preferredDevice = acc.ActiveDeviceID
	}

	err := c.addToQueueWithDevice(trackURI, preferredDevice)
	if err == nil {
		return nil
	}
	// Only transfer on 404; use play=true so playback resumes after transfer
	if strings.Contains(err.Error(), "404") {
		devices, devErr := c.GetPlayerDevices()
		if devErr != nil || len(devices) == 0 {
			return err
		}
		if preferredDevice != "" {
			for _, d := range devices {
				if d.ID == preferredDevice && !d.IsRestricted {
					_ = c.TransferPlayback(d.ID, true)
					if retryErr := c.addToQueueWithDevice(trackURI, d.ID); retryErr == nil {
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
			_ = c.TransferPlayback(d.ID, true)
			if retryErr := c.addToQueueWithDevice(trackURI, d.ID); retryErr == nil {
				return nil
			}
		}
	}
	return err
}

func (c *Client) addToQueueWithDevice(trackURI, deviceID string) error {
	q := url.Values{}
	q.Set("uri", trackURI)
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	resp, err := c.DoWithQuery(http.MethodPost, "/me/player/queue", q, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("add to queue: %s %s", resp.Status, string(body))
}

// GetRelatedArtists fetches artists similar to the given artist.
func (c *Client) GetRelatedArtists(artistID string) ([]string, error) {
	if artistID == "" {
		return nil, fmt.Errorf("artist ID required")
	}
	path := fmt.Sprintf("/artists/%s/related-artists", artistID)
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("related artists: %s %s", resp.Status, string(body))
	}
	var result struct {
		Artists []struct {
			ID string `json:"id"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Artists))
	for _, a := range result.Artists {
		if a.ID != "" {
			ids = append(ids, a.ID)
		}
	}
	return ids, nil
}

// GetArtistTopTracks fetches an artist's top tracks. Deprecated/removed for Dev Mode (Feb 2026); use GetTracksFromArtistAlbums as fallback.
func (c *Client) GetArtistTopTracks(artistID, market string) ([]Track, error) {
	if artistID == "" {
		return nil, fmt.Errorf("artist ID required")
	}
	if market == "" {
		market = "US"
	}
	path := fmt.Sprintf("/artists/%s/top-tracks?market=%s", artistID, url.QueryEscape(market))
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artist top tracks: %s %s", resp.Status, string(body))
	}
	var result struct {
		Tracks []Track `json:"tracks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Tracks, nil
}

// GetArtist fetches artist by ID. Used to get artist name for Search fallback.
func (c *Client) GetArtist(artistID string) (name string, err error) {
	if artistID == "" {
		return "", fmt.Errorf("artist ID required")
	}
	path := fmt.Sprintf("/artists/%s", artistID)
	resp, err := c.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("artist: %s %s", resp.Status, string(body))
	}
	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Name, nil
}

// SearchTracksByArtist searches for tracks by artist name. Fallback when Top Tracks/Albums return 403.
// Search limit is 10 for Dev Mode (Feb 2026).
func (c *Client) SearchTracksByArtist(artistName string, limit int) ([]Track, error) {
	if artistName == "" {
		return nil, fmt.Errorf("artist name required")
	}
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	q := url.Values{}
	q.Set("q", "artist:"+url.QueryEscape(artistName))
	q.Set("type", "track")
	q.Set("limit", fmt.Sprintf("%d", limit))
	path := "/search?" + q.Encode()
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search: %s %s", resp.Status, string(body))
	}
	var result struct {
		Tracks *struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Tracks == nil {
		return nil, nil
	}
	return result.Tracks.Items, nil
}

// GetArtistAlbums fetches an artist's albums. Still available per Feb 2026 (top-tracks was removed).
func (c *Client) GetArtistAlbums(artistID, market string, limit int) ([]string, error) {
	if artistID == "" {
		return nil, fmt.Errorf("artist ID required")
	}
	if market == "" {
		market = "US"
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	path := fmt.Sprintf("/artists/%s/albums?market=%s&limit=%d&include_groups=album,single", artistID, url.QueryEscape(market), limit)
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artist albums: %s %s", resp.Status, string(body))
	}
	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Items))
	for _, a := range result.Items {
		if a.ID != "" {
			ids = append(ids, a.ID)
		}
	}
	return ids, nil
}

// GetAlbumTracks fetches tracks from an album. Still available per Feb 2026.
func (c *Client) GetAlbumTracks(albumID string, limit int) ([]Track, error) {
	if albumID == "" {
		return nil, fmt.Errorf("album ID required")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	path := fmt.Sprintf("/albums/%s/tracks?limit=%d", albumID, limit)
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("album tracks: %s %s", resp.Status, string(body))
	}
	var result struct {
		Items []struct {
			ID          string `json:"id"`
			URI         string `json:"uri"`
			Name        string `json:"name"`
			DurationMs  int    `json:"duration_ms"`
			Artists     []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artists"`
			Album *struct {
				ID     string `json:"id"`
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"album"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(result.Items))
	for _, it := range result.Items {
		if it.ID == "" {
			continue
		}
		t := Track{
			ID:         it.ID,
			URI:        it.URI,
			Name:       it.Name,
			DurationMs: it.DurationMs,
		}
		for _, a := range it.Artists {
			t.Artists = append(t.Artists, struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}{ID: a.ID, Name: a.Name})
		}
		// Album omitted - album tracks API returns SimplifiedTrackObject without album
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// getTracksFromArtistAlbums fetches tracks via artist albums. Fallback when GetArtistTopTracks is removed (Dev Mode Feb 2026).
func (c *Client) getTracksFromArtistAlbums(artistID, market string, limit int) ([]Track, error) {
	albumIDs, err := c.GetArtistAlbums(artistID, market, 10)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var all []Track
	for i, albumID := range albumIDs {
		if i > 0 {
			time.Sleep(time.Duration(refillCallDelayMs) * time.Millisecond)
		}
		tracks, err := c.GetAlbumTracks(albumID, 20)
		if err != nil {
			log.Printf("spotify: GetAlbumTracks %s: %v", albumID, err)
			continue
		}
		for _, t := range tracks {
			if t.ID != "" && !seen[t.ID] {
				seen[t.ID] = true
				all = append(all, t)
				if len(all) >= limit {
					return all, nil
				}
			}
		}
	}
	return all, nil
}

// GetArtistIDsFromPlaylist extracts artist IDs from the first tracks in a playlist.
// Uses playlist items endpoint (avoids GET /tracks which returns 403 for new apps).
func (c *Client) GetArtistIDsFromPlaylist(playlistID string, maxTracks int) ([]string, error) {
	if maxTracks <= 0 {
		maxTracks = 5
	}
	seen := make(map[string]bool)
	var artistIDs []string
	offset := 0
	for len(artistIDs) < maxTracks*2 {
		path := fmt.Sprintf("/playlists/%s/items?limit=50&offset=%d", playlistID, offset)
		resp, err := c.Get(path)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("playlist items: %s %s", resp.Status, string(body))
		}
		var data struct {
			Items []struct {
				Track *struct {
					Artists []struct {
						ID string `json:"id"`
					} `json:"artists"`
				} `json:"track"`
				Item *struct {
					Artists []struct {
						ID string `json:"id"`
					} `json:"artists"`
				} `json:"item"`
			} `json:"items"`
			Next *string `json:"next"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		for _, pitem := range data.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track == nil {
				continue
			}
			for _, a := range track.Artists {
				if a.ID != "" && !seen[a.ID] {
					seen[a.ID] = true
					artistIDs = append(artistIDs, a.ID)
					if len(artistIDs) >= maxTracks*2 {
						break
					}
				}
			}
		}
		if data.Next == nil || *data.Next == "" {
			break
		}
		offset += 50
	}
	if len(artistIDs) == 0 {
		return nil, fmt.Errorf("no artist IDs found in playlist")
	}
	return artistIDs, nil
}

// fetchTracksForArtist fetches tracks for one artist (Top Tracks -> Albums fallback). Used by GetRefillTracksFromArtists.
func (c *Client) fetchTracksForArtist(artistID, market string, limit int) ([]Track, error) {
	tracks, err := c.GetArtistTopTracks(artistID, market)
	if err != nil {
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "404") {
			tracks, err = c.getTracksFromArtistAlbums(artistID, market, limit)
		}
		if err != nil {
			return nil, err
		}
	}
	return tracks, nil
}

// GetRefillTracksFromArtists fetches tracks from seed artist IDs (playlist artists).
// Uses parallel goroutines (refillConcurrency) to speed up. Top Tracks -> Albums -> Search fallback.
func (c *Client) GetRefillTracksFromArtists(seedArtistIDs []string, limit int) ([]Track, error) {
	if len(seedArtistIDs) == 0 {
		return nil, fmt.Errorf("seed artist IDs required")
	}
	if limit <= 0 {
		limit = 20
	}
	market := "US"
	seen := make(map[string]bool)
	tracksPerArtist := make(map[string]int)
	maxTracksPerArtist := 1
	var all []Track
	var mu sync.Mutex

	addTrack := func(t Track, sourceArtistID string) bool {
		mu.Lock()
		defer mu.Unlock()
		if t.ID == "" || seen[t.ID] {
			return false
		}
		artistID := sourceArtistID
		if artistID == "" && len(t.Artists) > 0 {
			artistID = t.Artists[0].ID
		}
		if artistID != "" && tracksPerArtist[artistID] >= maxTracksPerArtist {
			return false
		}
		seen[t.ID] = true
		if artistID != "" {
			tracksPerArtist[artistID]++
		}
		all = append(all, t)
		return true
	}

	// Parallel fetch: up to refillConcurrency artists at a time
	type result struct {
		artistID string
		tracks   []Track
		err      error
	}
	ids := make([]string, 0, len(seedArtistIDs))
	for _, id := range seedArtistIDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid seed artist IDs")
	}
	results := make(chan result, len(ids))
	sem := make(chan struct{}, refillConcurrency)

	for _, artistID := range ids {
		aid := artistID
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			tracks, err := c.fetchTracksForArtist(aid, market, limit*2)
			results <- result{artistID: aid, tracks: tracks, err: err}
		}()
	}

	received := 0
	for received < len(ids) {
		res := <-results
		received++
		if res.err != nil {
			log.Printf("spotify: fetch artist %s: %v", res.artistID, res.err)
			continue
		}
		for _, t := range res.tracks {
			if addTrack(t, res.artistID) {
				mu.Lock()
				enough := len(all) >= limit*2
				mu.Unlock()
				if enough {
					goto done
				}
			}
		}
	}
done:

	// Fallback: Search by artist name when all parallel fetches failed
	if len(all) == 0 {
		log.Printf("spotify: top tracks/albums failed, trying Search by artist name")
		for i, artistID := range seedArtistIDs {
			if i > 0 {
				time.Sleep(time.Duration(refillCallDelayMs) * time.Millisecond)
			}
			name, err := c.GetArtist(artistID)
			if err != nil {
				log.Printf("spotify: GetArtist %s: %v", artistID, err)
				continue
			}
			time.Sleep(time.Duration(refillCallDelayMs) * time.Millisecond)
			tracks, err := c.SearchTracksByArtist(name, 10)
			if err != nil {
				log.Printf("spotify: SearchTracksByArtist %s: %v", name, err)
				continue
			}
			for _, t := range tracks {
				if addTrack(t, artistID) {
					mu.Lock()
					enough := len(all) >= limit*2
					mu.Unlock()
					if enough {
						break
					}
				}
			}
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no tracks from artists")
	}
	return all, nil
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

	// Feb 2026: use /items (POST /playlists/{id}/tracks deprecated)
	path := fmt.Sprintf("/playlists/%s/items", playlistID)
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

// RemoveTracksFromPlaylist removes tracks from a playlist by URI
func (c *Client) RemoveTracksFromPlaylist(playlistID string, trackURIs []string) error {
	if len(trackURIs) == 0 {
		return nil
	}
	// Feb 2026: use /items with "items" body (DELETE /playlists/{id}/tracks deprecated)
	items := make([]map[string]string, 0, len(trackURIs))
	for _, uri := range trackURIs {
		items = append(items, map[string]string{"uri": uri})
	}
	body := map[string]interface{}{"items": items}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/playlists/%s/items", playlistID)
	resp, err := c.Do(http.MethodDelete, path, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove tracks from playlist: %s %s", resp.Status, string(bodyBytes))
	}
	return nil
}

// CreatePlaylist creates a playlist for the current user (POST /v1/me/playlists).
// Uses /me so the path does not depend on stored user id (avoids 403 from user id mismatch).
func (c *Client) CreatePlaylist(name, description string, public bool) (string, error) {
	if name == "" {
		return "", fmt.Errorf("playlist name required")
	}
	body := map[string]interface{}{
		"name":        name,
		"description": description,
		"public":      public,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	resp, err := c.Post("/me/playlists", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		s := string(b)
		if resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("create playlist: %s %s — open the app, disconnect Spotify, and connect again so the token includes playlist-modify scopes", resp.Status, s)
		}
		return "", fmt.Errorf("create playlist: %s %s", resp.Status, s)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("create playlist: empty id in response")
	}
	return out.ID, nil
}

// SearchTracks runs a general track search (Spotify Web API search).
func (c *Client) SearchTracks(query string, limit int) ([]Track, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query required")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("type", "track")
	q.Set("limit", strconv.Itoa(limit))
	path := "/search?" + q.Encode()
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search: %s %s", resp.Status, string(body))
	}
	var result struct {
		Tracks *struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Tracks == nil {
		return nil, nil
	}
	return result.Tracks.Items, nil
}
