package models

// SessionRefillTrack records a vibe-fill track added during a voting session (for cleanup on session end or process restart).
type SessionRefillTrack struct {
	ID              uint   `gorm:"primaryKey"`
	VotingSessionID uint   `gorm:"index;not null"`
	TrackID         string `gorm:"size:64;not null"`
	TrackURI        string `gorm:"size:255;not null"`
}

func (SessionRefillTrack) TableName() string {
	return "session_refill_tracks"
}
