package models

import "time"

// MajorSyncJob 記錄每次主力進出批次爬取作業的狀態
type MajorSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement"  json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"           json:"status"` // running|completed|failed
	Days        int        `gorm:"default:1"                 json:"days"`   // 1=今日, 5=近5日, ...
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (MajorSyncJob) TableName() string { return "major_sync_jobs" }

// MajorBrokerRecord 記錄某支股票在特定日期、特定天期的主力券商進出明細。
// 唯一索引：(symbol, data_date, days, side, rank)
type MajorBrokerRecord struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"                               json:"id"`
	JobID      uint      `gorm:"index"                                                  json:"job_id"`
	Symbol     string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_major_record"  json:"symbol"`
	DataDate   time.Time `gorm:"type:date;not null;uniqueIndex:idx_major_record"         json:"data_date"`
	Days       int       `gorm:"not null;uniqueIndex:idx_major_record"                   json:"days"`
	Side       string    `gorm:"type:varchar(4);not null;uniqueIndex:idx_major_record"   json:"side"` // buy|sell
	Rank       int       `gorm:"not null;uniqueIndex:idx_major_record"                   json:"rank"` // 1-10
	BrokerName string    `gorm:"type:varchar(60);not null"                              json:"broker_name"`
	BuyVol     int       `gorm:"not null"                                               json:"buy_vol"`
	SellVol    int       `gorm:"not null"                                               json:"sell_vol"`
	NetVol     int       `gorm:"not null"                                               json:"net_vol"` // 正=買超, 負=賣超
	Pct        float64   `gorm:"type:numeric(7,4)"                                      json:"pct"`     // 佔成交比重(%)
	ScrapedAt  time.Time `gorm:"not null"                                               json:"scraped_at"`
}

func (MajorBrokerRecord) TableName() string { return "major_broker_records" }
