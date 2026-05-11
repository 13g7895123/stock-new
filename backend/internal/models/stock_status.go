package models

import "time"

const (
	StockStatusDisposition        = "disposition"
	StockStatusAttention          = "attention"
	StockStatusDayTradeRestricted = "day_trade_restricted"
)

type StockStatus struct {
	ID         uint      `json:"id" gorm:"primarykey;autoIncrement"`
	Symbol     string    `json:"symbol" gorm:"size:16;not null;uniqueIndex:idx_stock_status_identity"`
	Name       string    `json:"name" gorm:"size:80;not null"`
	Market     string    `json:"market" gorm:"size:8;not null;uniqueIndex:idx_stock_status_identity"`
	Type       string    `json:"type" gorm:"size:32;not null;uniqueIndex:idx_stock_status_identity;index"`
	SourceDate time.Time `json:"source_date" gorm:"type:date;not null;index"`
	StartDate  time.Time `json:"start_date" gorm:"type:date;not null;uniqueIndex:idx_stock_status_identity;index"`
	EndDate    time.Time `json:"end_date" gorm:"type:date;not null;uniqueIndex:idx_stock_status_identity;index"`
	Reason     string    `json:"reason" gorm:"type:text"`
	Measure    string    `json:"measure" gorm:"type:text"`
	Detail     string    `json:"detail" gorm:"type:text"`
	RawPeriod  string    `json:"raw_period" gorm:"type:text"`
	SourceURL  string    `json:"source_url" gorm:"type:text"`
	FetchedAt  time.Time `json:"fetched_at" gorm:"not null;index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (StockStatus) TableName() string { return "stock_statuses" }

type StockStatusSyncJob struct {
	ID          uint       `json:"id" gorm:"primarykey;autoIncrement"`
	StartedAt   time.Time  `json:"started_at" gorm:"not null"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status" gorm:"size:20;not null;index"`
	Total       int        `json:"total"`
	Message     string     `json:"message" gorm:"type:text"`
}

func (StockStatusSyncJob) TableName() string { return "stock_status_sync_jobs" }
