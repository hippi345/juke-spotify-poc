package models

import (
	"time"
)

// SpotifyAccount stores OAuth tokens for a connected Spotify user
type SpotifyAccount struct {
	ID              uint      `gorm:"primaryKey"`
	SpotifyUserID   string    `gorm:"uniqueIndex;size:255;not null"`
	DisplayName     string    `gorm:"size:255"`
	RefreshToken    string    `gorm:"type:text;not null"`
	AccessToken     string    `gorm:"type:text"`
	TokenExpiresAt  time.Time `gorm:"not null"`
	ActiveDeviceID  string    `gorm:"size:255"` // Preferred playback device
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName overrides the table name
func (SpotifyAccount) TableName() string {
	return "spotify_accounts"
}
