package db

import (
	"fmt"
	"log"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB holds the GORM database instance
var DB *gorm.DB

// SetDB replaces the global DB (for tests)
func SetDB(database *gorm.DB) {
	DB = database
}

// Connect establishes a connection to MySQL using config
func Connect(cfg *config.Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=10s&writeTimeout=10s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	const attempts = 30
	var lastErr error
	for i := 1; i <= attempts; i++ {
		var err error
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		if i < attempts {
			log.Printf("MySQL not ready (attempt %d/%d): %v", i, attempts, err)
			time.Sleep(2 * time.Second)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", lastErr)
	}

	log.Println("Connected to MySQL successfully")

	if err := DB.AutoMigrate(
		&models.SpotifyAccount{},
		&models.User{},
		&models.Venue{},
		&models.VotingSession{},
		&models.PaidSkip{},
		&models.SessionRefillTrack{},
	); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}
