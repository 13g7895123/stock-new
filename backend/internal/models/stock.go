package models

import (
	"time"

	"gorm.io/gorm"
)

type Stock struct {
	ID        uint           `json:"id"         gorm:"primarykey"`
	Symbol    string         `json:"symbol"     gorm:"uniqueIndex;not null"`
	Name      string         `json:"name"       gorm:"not null"`
	Industry  string         `json:"industry"   gorm:"default:''"`
	Market    string         `json:"market"     gorm:"default:''"` // TWSE | TPEX
	Price     float64        `json:"price"`
	Change    float64        `json:"change"`
	ChangePct float64        `json:"change_pct"`
	Volume    int64          `json:"volume"`
	Tags      []Tag          `json:"tags"       gorm:"many2many:stock_tags;"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`
}

// Tag 使用者自訂標籤
type Tag struct {
	ID        uint           `json:"id"        gorm:"primarykey"`
	Name      string         `json:"name"      gorm:"uniqueIndex;not null"`
	Color     string         `json:"color"     gorm:"default:'#6b7280'"` // hex color
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"         gorm:"index"`
}
