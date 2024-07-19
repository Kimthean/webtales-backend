package db

import (
	"go-novel/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDB(databaseURL string) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
    if err != nil {
        return nil, err
    }

    // Auto Migrate the schema
    db.AutoMigrate(&models.Novel{}, &models.Chapter{})

    return db, nil
}
