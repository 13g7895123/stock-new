package models

import "time"

// InstitutionalSyncJob 記錄每次三大法人批次爬取作業的狀態
type InstitutionalSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"          json:"status"` // running|completed|failed
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (InstitutionalSyncJob) TableName() string { return "institutional_sync_jobs" }

// InstitutionalTrading 儲存每日三大法人（外資、投信、自營商）買賣超資料。
// 唯一索引：(date, symbol)
type InstitutionalTrading struct {
	ID     uint      `gorm:"primaryKey;autoIncrement"                                   json:"id"`
	JobID  uint      `gorm:"index"                                                      json:"job_id"`
	Symbol string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_inst_symbol_date" json:"symbol"`
	Date   time.Time `gorm:"type:date;not null;uniqueIndex:idx_inst_symbol_date"        json:"date"`
	Market string    `gorm:"type:varchar(8);not null"                                   json:"market"` // TWSE|TPEX

	// 外資（不含外資自營商）
	ForeignBuy  int64 `gorm:"not null;default:0" json:"foreign_buy"`  // 外資買進股數
	ForeignSell int64 `gorm:"not null;default:0" json:"foreign_sell"` // 外資賣出股數
	ForeignNet  int64 `gorm:"not null;default:0" json:"foreign_net"`  // 外資買賣超股數

	// 投信
	TrustBuy  int64 `gorm:"not null;default:0" json:"trust_buy"`  // 投信買進股數
	TrustSell int64 `gorm:"not null;default:0" json:"trust_sell"` // 投信賣出股數
	TrustNet  int64 `gorm:"not null;default:0" json:"trust_net"`  // 投信買賣超股數

	// 自營商合計
	DealerNet int64 `gorm:"not null;default:0" json:"dealer_net"` // 自營商買賣超股數（自行+避險）

	// 三大法人合計
	TotalNet int64 `gorm:"not null;default:0" json:"total_net"` // 三大法人合計買賣超股數
}

func (InstitutionalTrading) TableName() string { return "institutional_trading" }
