package models

import "time"

// ChipsSyncJob 記錄每次籌碼金字塔爬取作業的狀態
type ChipsSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"         json:"status"` // running|completed|failed|never
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (ChipsSyncJob) TableName() string { return "chips_sync_jobs" }
