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

// ChipsHolderSnapshot 儲存單一股票某次資料日期的籌碼快照。
type ChipsHolderSnapshot struct {
	ID            uint                      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID         uint                      `gorm:"index" json:"job_id"`
	Symbol        string                    `gorm:"type:varchar(10);not null;uniqueIndex:idx_chips_symbol_date" json:"symbol"`
	DataDate      time.Time                 `gorm:"type:date;not null;uniqueIndex:idx_chips_symbol_date" json:"data_date"`
	ScrapedAt     time.Time                 `gorm:"not null" json:"scraped_at"`
	Distributions []ChipsHolderDistribution `gorm:"foreignKey:SnapshotID;constraint:OnDelete:CASCADE" json:"distributions,omitempty"`
}

func (ChipsHolderSnapshot) TableName() string { return "chips_holder_snapshots" }

// ChipsHolderDistribution 儲存單一快照的持股區間明細。
type ChipsHolderDistribution struct {
	ID           uint     `gorm:"primaryKey;autoIncrement" json:"id"`
	SnapshotID   uint     `gorm:"not null;index" json:"snapshot_id"`
	TierRank     int      `gorm:"not null" json:"tier_rank"`
	RangeLabel   string   `gorm:"type:varchar(60);not null" json:"range_label"`
	HolderCount  *int     `json:"holder_count"`
	HolderPct    *float64 `gorm:"type:numeric(7,4)" json:"holder_pct"`
	ShareCount   *int64   `json:"share_count"`
	SharePct     *float64 `gorm:"type:numeric(7,4)" json:"share_pct"`
	CumHolderPct *float64 `gorm:"type:numeric(7,4)" json:"cum_holder_pct"`
	CumSharePct  *float64 `gorm:"type:numeric(7,4)" json:"cum_share_pct"`
}

func (ChipsHolderDistribution) TableName() string { return "chips_holder_distributions" }

// ChipsRunLog 儲存籌碼金字塔每次執行流程的詳細診斷日誌，
// 供後台 GET /api/chips/logs 查詢，協助排查爬取失敗原因。
type ChipsRunLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID     *uint     `gorm:"index"                    json:"job_id"` // nil = handler 層（job 尚未建立）
	Level     string    `gorm:"size:10;default:'info'"   json:"level"`  // info | warn | error
	Step      string    `gorm:"size:60"                  json:"step"`   // dispatch | fetch_retry | fetch_fail | parse_fail | save_fail | job_start | job_end
	Symbol    string    `gorm:"size:10"                  json:"symbol"` // 空字串 = job 層級
	Message   string    `gorm:"type:text"                json:"message"`
	CreatedAt time.Time `gorm:"index"                    json:"created_at"`
}

func (ChipsRunLog) TableName() string { return "chips_run_logs" }

// PriceSyncJob 記錄每次「全股票所有歷史日K」批次爬取作業的狀態
type PriceSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"         json:"status"` // running|completed|failed
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (PriceSyncJob) TableName() string { return "price_sync_jobs" }
