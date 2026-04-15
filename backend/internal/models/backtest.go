package models

import "time"

// BacktestStrategy 使用者儲存的回測策略
type BacktestStrategy struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Params      string    `gorm:"type:jsonb;default:'{}'" json:"params"` // 策略參數 JSON
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (BacktestStrategy) TableName() string { return "backtest_strategies" }

// BacktestJob 每次回測執行記錄
type BacktestJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StrategyID  uint       `gorm:"index" json:"strategy_id"`
	Symbol      string     `gorm:"type:varchar(10)" json:"symbol"` // 空=全市場
	StartDate   string     `gorm:"type:date" json:"start_date"`
	EndDate     string     `gorm:"type:date" json:"end_date"`
	Capital     float64    `gorm:"default:1000000" json:"capital"` // 初始資金（元）
	Params      string     `gorm:"type:jsonb;default:'{}'" json:"params"`
	Status      string     `gorm:"default:'pending'" json:"status"` // pending|running|completed|failed
	Progress    int        `json:"progress"`                        // 0~100
	Message     string     `json:"message"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	// 彙總結果
	TotalReturn  float64 `json:"total_return"`  // 總報酬率 %
	AnnualReturn float64 `json:"annual_return"` // 年化報酬率 %
	MaxDrawdown  float64 `json:"max_drawdown"`  // 最大回撤 %
	WinRate      float64 `json:"win_rate"`      // 勝率 %
	SharpeRatio  float64 `json:"sharpe_ratio"`
	TotalTrades  int     `json:"total_trades"`
}

func (BacktestJob) TableName() string { return "backtest_jobs" }

// BacktestTrade 每筆交易記錄
type BacktestTrade struct {
	ID         uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID      uint    `gorm:"index;not null" json:"job_id"`
	Symbol     string  `gorm:"type:varchar(10)" json:"symbol"`
	EntryDate  string  `gorm:"type:date" json:"entry_date"`
	ExitDate   string  `gorm:"type:date" json:"exit_date"`
	EntryPrice float64 `json:"entry_price"`
	ExitPrice  float64 `json:"exit_price"`
	Shares     int64   `json:"shares"`  // 股數
	PnL        float64 `json:"pnl"`     // 損益（元）
	PnLPct     float64 `json:"pnl_pct"` // 損益 %
	HoldDays   int     `json:"hold_days"`
	ExitReason string  `json:"exit_reason"` // stop_loss|take_profit|end_date|signal
}

func (BacktestTrade) TableName() string { return "backtest_trades" }

// BacktestEquityPoint 權益曲線（每日）
type BacktestEquityPoint struct {
	ID     uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID  uint    `gorm:"index;not null" json:"job_id"`
	Date   string  `gorm:"type:date" json:"date"`
	Equity float64 `json:"equity"` // 當日總資產
	Cash   float64 `json:"cash"`
}

func (BacktestEquityPoint) TableName() string { return "backtest_equity_points" }
