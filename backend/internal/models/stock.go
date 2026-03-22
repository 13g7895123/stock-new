package models

import (
	"time"

	"gorm.io/gorm"
)

type Stock struct {
	ID        uint           `json:"id"         gorm:"primarykey"`
	Symbol    string         `json:"symbol"     gorm:"uniqueIndex;not null"`
	Name      string         `json:"name"       gorm:"not null"`
	Price     float64        `json:"price"`
	Change    float64        `json:"change"`
	ChangePct float64        `json:"change_pct"`
	Volume    int64          `json:"volume"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`
}
