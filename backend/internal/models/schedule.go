package models

import "time"

// TaskSchedule 儲存各任務的排程設定。
// Type:
//   - "manual"  僅手動觸發，停用自動排程
//   - "daily"   每天 Hour:Minute 觸發
//   - "weekly"  每週 Weekday 的 Hour:Minute 觸發
type TaskSchedule struct {
	TaskID    string     `gorm:"primaryKey;size:50" json:"task_id"`
	Enabled   bool       `gorm:"default:false"      json:"enabled"`
	Type      string     `gorm:"size:20;default:'manual'" json:"type"` // manual|daily|weekly
	Hour      int        `gorm:"default:10"         json:"hour"`       // 0-23
	Minute    int        `gorm:"default:0"          json:"minute"`     // 0-59
	Weekday   int        `gorm:"default:0"          json:"weekday"`    // 0=Sun..6=Sat（僅 Type=weekly 使用）
	Params    string     `gorm:"type:text;default:'{}'" json:"params"` // JSON 額外參數，例 {"days":1}
	LastRunAt *time.Time `json:"last_run_at"`
	NextRunAt *time.Time `json:"next_run_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (TaskSchedule) TableName() string { return "task_schedules" }
