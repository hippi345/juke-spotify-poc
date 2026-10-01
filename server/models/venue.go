package models

import "time"

// Venue is a physical location owned by a staff user.
type Venue struct {
	ID          uint    `gorm:"primaryKey"`
	StaffUserID uint    `gorm:"index;not null"`
	Name        string  `gorm:"size:255;not null"`
	Latitude    float64 `gorm:"not null"`
	Longitude   float64 `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Venue) TableName() string {
	return "venues"
}
