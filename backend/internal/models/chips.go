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
