package models

import (
	"time"
)

// VotingSession represents an active or ended voting session
type VotingSession struct {
	ID              uint      `gorm:"primaryKey"`
	PlaylistID      string    `gorm:"size:255;not null"`
	PlaylistName    string    `gorm:"size:255"`
	RefillThreshold int      `gorm:"default:0"` // Refill when unused tracks < this (0 = disabled)
	Status          string    `gorm:"size:32;not null"` // "active" | "ended"
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName overrides the table name
func (VotingSession) TableName() string {
	return "voting_sessions"
}
