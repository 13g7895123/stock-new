package models

import "time"

// Dividend 記錄每支股票的除息（現金股利）與除權（股票股利）記錄
// 唯一索引：(symbol, ex_date)
type Dividend struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"                                      json:"id"`
	Symbol        string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_div_symbol_date"     json:"symbol"`
	ExDate        time.Time `gorm:"type:date;not null;uniqueIndex:idx_div_symbol_date"            json:"ex_date"`
	CashDividend  float64   `gorm:"type:numeric(8,4);not null;default:0"                          json:"cash_dividend"`  // 現金股利（元）
	StockDividend float64   `gorm:"type:numeric(8,4);not null;default:0"                          json:"stock_dividend"` // 股票股利（元）
	RefPrice      float64   `gorm:"type:numeric(10,2);not null;default:0"                         json:"ref_price"`      // 除息參考價（元）
	Market        string    `gorm:"type:varchar(8);not null;default:'TWSE'"                       json:"market"`         // TWSE|TPEX
	CreatedAt     time.Time `json:"created_at"`
}

func (Dividend) TableName() string { return "dividends" }

// DividendSyncJob 記錄每次除息資訊批次爬取作業狀態
type DividendSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"         json:"status"` // running|completed|failed
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (DividendSyncJob) TableName() string { return "dividend_sync_jobs" }
