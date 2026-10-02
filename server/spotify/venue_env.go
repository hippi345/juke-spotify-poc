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

// BootstrapVenueAccountFromEnv connects the venue Spotify account when
// SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET, and SPOTIFY_REFRESH_TOKEN are set.
// Used by Compose session walk CI and local automation without OAuth in a browser.
func BootstrapVenueAccountFromEnv(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	clientID := strings.TrimSpace(cfg.SpotifyClientID)
	clientSecret := strings.TrimSpace(cfg.SpotifyClientSecret)
	refresh := strings.TrimSpace(cfg.SpotifyVenueRefreshToken)
	if clientID == "" || clientSecret == "" || refresh == "" {
		return nil
	}

	tokenResp, err := refreshAccessToken(clientID, clientSecret, refresh)
	if err != nil {
		return fmt.Errorf("spotify venue bootstrap: %w", err)
	}

	profile, err := fetchUserProfile(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("spotify venue bootstrap profile: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	acc := &models.SpotifyAccount{
		SpotifyUserID:  profile.ID,
		DisplayName:    profile.DisplayName,
		RefreshToken:   refresh,
		AccessToken:    tokenResp.AccessToken,
		TokenExpiresAt: expiresAt,
	}
	if tokenResp.RefreshToken != "" {
		acc.RefreshToken = tokenResp.RefreshToken
	}

	var existing models.SpotifyAccount
	err = db.DB.Where("spotify_user_id = ?", profile.ID).First(&existing).Error
	if err == nil {
		existing.RefreshToken = acc.RefreshToken
		existing.AccessToken = acc.AccessToken
		existing.TokenExpiresAt = acc.TokenExpiresAt
		existing.DisplayName = acc.DisplayName
		if err := db.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("spotify: venue account refreshed from environment for user %s", profile.ID)
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	if err := db.DB.Create(acc).Error; err != nil {
		return err
	}
	log.Printf("spotify: venue account connected from environment for user %s", profile.ID)
	return nil
}

func refreshAccessToken(clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest(http.MethodPost, spotifyAuthURL+"/api/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh: %s %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token refresh: empty access_token")
	}
	return &tokenResp, nil
}

func fetchUserProfile(accessToken string) (*UserProfile, error) {
	req, err := http.NewRequest(http.MethodGet, spotifyAPIURL+"/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("profile: %s %s", resp.Status, string(body))
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("profile: empty user id")
	}
	return &profile, nil
}
