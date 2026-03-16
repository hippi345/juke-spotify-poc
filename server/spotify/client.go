package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
