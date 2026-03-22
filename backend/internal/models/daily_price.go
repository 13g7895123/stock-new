package models

import "time"

// DailyPrice 儲存每日 OHLCV 資料（上市 + 上櫃）
type DailyPrice struct {
	ID      uint      `json:"id"      gorm:"primarykey;autoIncrement"`
	Symbol  string    `json:"symbol"  gorm:"not null;uniqueIndex:idx_daily_symbol_date"`
	Date    time.Time `json:"date"    gorm:"not null;type:date;uniqueIndex:idx_daily_symbol_date"`
	Open    float64   `json:"open"    gorm:"not null"`
	High    float64   `json:"high"    gorm:"not null"`
	Low     float64   `json:"low"     gorm:"not null"`
	Close   float64   `json:"close"   gorm:"not null"`
	Volume  int64     `json:"volume"  gorm:"not null"` // 成交量（股）
	TxValue int64     `json:"tx_value"`                // 成交金額（元）
	TxCount int       `json:"tx_count"`                // 成交筆數
}

func (DailyPrice) TableName() string { return "daily_prices" }
