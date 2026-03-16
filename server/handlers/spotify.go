package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
)

// SpotifyLogin initiates OAuth by redirecting to Spotify
func SpotifyLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginURL, err := spotify.LoginRedirect(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Support JSON response for client-side redirect (allows error display)
		if c.GetHeader("Accept") == "application/json" {
			c.JSON(http.StatusOK, gin.H{"url": loginURL})
			return
		}
		c.Redirect(http.StatusFound, loginURL)
	}
}

// SpotifyCallback handles the OAuth callback, exchanges code for tokens, stores account
func SpotifyCallback(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errParam := c.Query("error")

		if errParam != "" {
			c.Redirect(http.StatusFound, cfg.AppBaseURL+"?spotify=denied")
			return
		}

		if !spotify.ValidateAndConsumeState(state) {
			c.Redirect(http.StatusFound, cfg.AppBaseURL+"?spotify=state_mismatch")
			return
		}

		if code == "" {
			c.Redirect(http.StatusFound, cfg.AppBaseURL+"?spotify=no_code")
			return
		}

		_, err := spotify.ExchangeCode(cfg, code)
		if err != nil {
			c.Redirect(http.StatusFound, cfg.AppBaseURL+"?spotify=error")
			return
		}

		c.Redirect(http.StatusFound, cfg.AppBaseURL+"?spotify=connected")
	}
}

// SpotifyStatus returns whether a Spotify account is connected
func SpotifyStatus(svc *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc, err := svc.GetDefaultAccount()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if acc == nil {
			c.JSON(http.StatusOK, gin.H{"connected": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"connected":    true,
			"display_name": acc.DisplayName,
			"spotify_id":   acc.SpotifyUserID,
		})
	}
}

// SpotifyMe fetches the current user's profile from Spotify API (test endpoint)
func SpotifyMe(svc *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := svc.Get("/me")
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
			return
		}

		var profile map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}
