package models

import "time"

// AppSetting 是 key-value 設定表，存各功能的爬取方案配置。
type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:100" json:"key"`
	Value     string    `gorm:"type:text"          json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppSetting) TableName() string { return "app_settings" }
