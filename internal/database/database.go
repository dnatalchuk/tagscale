// internal/database/database.go
package database

import (
	"strings"
	"tagscale/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect(databaseURL string) (*gorm.DB, error) {
	if strings.HasPrefix(databaseURL, "sqlite://") || strings.HasPrefix(databaseURL, "file:") {
		path := strings.TrimPrefix(databaseURL, "sqlite://")
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.CostRecord{},
		&models.CostAnalysis{},
		&models.TeamMapping{},
	)
}
