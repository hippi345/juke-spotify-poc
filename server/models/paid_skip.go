package models

import "time"

// PaidSkip records a patron Stripe payment to queue a playlist track ahead of the vote winner.
type PaidSkip struct {
	ID                      uint       `gorm:"primaryKey" json:"id"`
	VotingSessionID         uint       `gorm:"index;not null" json:"voting_session_id"`
	PatronUserID            uint       `gorm:"index;not null" json:"patron_user_id"`
	TrackID                 string     `gorm:"size:64;not null" json:"track_id"`
	TrackURI                string     `gorm:"size:256;not null" json:"track_uri"`
	StripeCheckoutSessionID string     `gorm:"size:128;uniqueIndex" json:"stripe_checkout_session_id"`
	Status                  string     `gorm:"size:32;not null;default:pending" json:"status"` // pending, paid, failed
	PaidAt                  *time.Time `json:"paid_at,omitempty"`
	QueuedAt                *time.Time `json:"queued_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}
