package db

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(databaseURL string) (*gorm.DB, error) {

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:                 newLogger,
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// err = db.AutoMigrate(&models.Novel{}, &models.Chapter{})
	// if err != nil {
	// 	return nil, err
	// }

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(3)

	sqlDB.SetMaxOpenConns(100)

	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}
