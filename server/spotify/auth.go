package spotify

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
)

// Scopes for playlist, queue, and playback management
// Note: playlist-read-public is not a valid Spotify scope; use playlist-read-private for owned playlists
const scopes = "playlist-read-private playlist-read-collaborative playlist-modify-public playlist-modify-private user-modify-playback-state user-read-playback-state user-read-currently-playing user-read-private"

var (
	stateStore   = make(map[string]time.Time)
	stateStoreMu sync.RWMutex
)

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:22], nil
}

func storeState(state string) {
	stateStoreMu.Lock()
	defer stateStoreMu.Unlock()
	stateStore[state] = time.Now()
}

// ValidateAndConsumeState checks state and removes it (one-time use)
func ValidateAndConsumeState(state string) bool {
	stateStoreMu.Lock()
	defer stateStoreMu.Unlock()
	if _, ok := stateStore[state]; !ok {
		return false
	}
	delete(stateStore, state)
	return true
}

// LoginRedirect builds the Spotify authorization URL and returns it for redirect
func LoginRedirect(cfg *config.Config) (string, error) {
	if cfg.SpotifyClientID == "" || cfg.SpotifyClientSecret == "" {
		return "", fmt.Errorf("SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET must be set")
	}

	state, err := generateState()
	if err != nil {
		return "", err
	}
	storeState(state)

	params := url.Values{}
	params.Set("client_id", cfg.SpotifyClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", cfg.SpotifyRedirectURI)
	params.Set("scope", scopes)
	params.Set("state", state)

	return "https://accounts.spotify.com/authorize?" + params.Encode(), nil
}

// TokenResponse from Spotify token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// UserProfile from Spotify /me
type UserProfile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// ExchangeCode exchanges authorization code for tokens and fetches user profile
func ExchangeCode(cfg *config.Config, code string) (*models.SpotifyAccount, error) {
	tokenURL := "https://accounts.spotify.com/api/token"
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", cfg.SpotifyRedirectURI)

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.SpotifyClientID, cfg.SpotifyClientSecret)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Fetch user profile
	profileReq, _ := http.NewRequest(http.MethodGet, "https://api.spotify.com/v1/me", nil)
	profileReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	profileResp, err := client.Do(profileReq)
	if err != nil {
		return nil, err
	}
	defer profileResp.Body.Close()

	if profileResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(profileResp.Body)
		return nil, fmt.Errorf("profile fetch failed: %s %s", profileResp.Status, string(body))
	}

	var profile UserProfile
	if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	acc := &models.SpotifyAccount{
		SpotifyUserID:  profile.ID,
		DisplayName:    profile.DisplayName,
		RefreshToken:   tokenResp.RefreshToken,
		AccessToken:    tokenResp.AccessToken,
		TokenExpiresAt: expiresAt,
	}

	// Upsert: update if exists, create if not
	var existing models.SpotifyAccount
	err = db.DB.Where("spotify_user_id = ?", profile.ID).First(&existing).Error
	if err == nil {
		existing.RefreshToken = acc.RefreshToken
		existing.AccessToken = acc.AccessToken
		existing.TokenExpiresAt = acc.TokenExpiresAt
		existing.DisplayName = acc.DisplayName
		if err := db.DB.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	if err := db.DB.Create(acc).Error; err != nil {
		return nil, err
	}
	return acc, nil
}
