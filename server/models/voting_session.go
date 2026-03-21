package models

import (
	"time"
)

// VotingSession stores a voting session
type VotingSession struct {
	ID              uint      `gorm:"primaryKey"`
	PlaylistID      string    `gorm:"size:255;not null"`
	PlaylistName    string    `gorm:"size:255"`
	RefillThreshold int       `gorm:"not null;default:0"` // Refill when (total - played) < this
	RefillCount     int       `gorm:"not null;default:20"` // Number of tracks to add when refilling
	Status          string    `gorm:"size:32;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName overrides the table name
func (VotingSession) TableName() string {
	return "voting_sessions"
}
