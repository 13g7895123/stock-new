package models

import "time"

// BrokerTradeEvent 記錄某券商在某股票的一次「建倉→出場」事件。
// 唯一索引：(symbol, broker_name, entry_date)
type BrokerTradeEvent struct {
	ID          uint       `gorm:"primaryKey;autoIncrement"                                     json:"id"`
	Symbol      string     `gorm:"type:varchar(10);not null;uniqueIndex:idx_bte_unique"         json:"symbol"`
	BrokerName  string     `gorm:"type:varchar(60);not null;uniqueIndex:idx_bte_unique"         json:"broker_name"`
	EntryDate   time.Time  `gorm:"type:date;not null;uniqueIndex:idx_bte_unique"                json:"entry_date"`
	EntryClose  float64    `gorm:"type:numeric(10,2);not null"                                  json:"entry_close"`
	EntryNetVol int        `gorm:"not null"                                                     json:"entry_net_vol"`
	ExitDate    *time.Time `gorm:"type:date"                                                    json:"exit_date"`
	ExitClose   *float64   `gorm:"type:numeric(10,2)"                                           json:"exit_close"`
	ExitNetVol  *int       `                                                                    json:"exit_net_vol"`
	ReturnPct   *float64   `gorm:"type:numeric(8,4)"                                            json:"return_pct"`
	HoldingDays *int       `                                                                    json:"holding_days"`
	IsWin       *bool      `                                                                    json:"is_win"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (BrokerTradeEvent) TableName() string { return "broker_trade_events" }

// WinrateSyncJob 記錄每次券商勝率批次計算作業的狀態
type WinrateSyncJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"         json:"status"` // running|completed|failed
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (WinrateSyncJob) TableName() string { return "winrate_sync_jobs" }
