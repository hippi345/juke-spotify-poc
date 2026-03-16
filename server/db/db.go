package db

import (
	"fmt"
	"log"

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
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	log.Println("Connected to MySQL successfully")

	if err := DB.AutoMigrate(&models.SpotifyAccount{}); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}
