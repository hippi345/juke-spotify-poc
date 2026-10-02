package models

import "time"

// User is a staff or patron account (email + password).
type User struct {
	ID                    uint   `gorm:"primaryKey"`
	Email                 string `gorm:"uniqueIndex;size:255;not null"`
	PasswordHash          string `gorm:"size:255;not null"`
	Role                  string `gorm:"size:32;not null"` // staff | patron
	JoinedVotingSessionID *uint  `gorm:"index"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (User) TableName() string {
	return "users"
}
