package database

import (
	"fmt"

	"stock-backend/internal/config"
	"stock-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Taipei",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&models.Stock{},
		&models.Tag{},
		&models.DailyPrice{},
		&models.ChipsSyncJob{},
		&models.ChipsHolderSnapshot{},
		&models.ChipsHolderDistribution{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
