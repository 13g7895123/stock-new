package models

import "time"

// StockChipScore 記錄每支股票的最新籌碼評分（每日收盤後計算）
// 唯一索引：symbol（每支股票只保存最近一次評分）
type StockChipScore struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement"       json:"id"`
	Symbol             string    `gorm:"type:varchar(10);not null;uniqueIndex" json:"symbol"`
	CalcDate           time.Time `gorm:"type:date;not null"             json:"calc_date"`
	TotalScore         float64   `gorm:"type:numeric(5,2);not null"     json:"total_score"`         // 0~100
	InstitutionalScore float64   `gorm:"type:numeric(5,2);not null"     json:"institutional_score"` // 三大法人面（35%）
	MajorScore         float64   `gorm:"type:numeric(5,2);not null"     json:"major_score"`         // 主力券商面（35%）
	ChipsPyramidScore  float64   `gorm:"type:numeric(5,2);not null"     json:"chips_pyramid_score"` // 大戶持股面（15%）
	WinrateScore       float64   `gorm:"type:numeric(5,2);not null"     json:"winrate_score"`       // 勝率面（15%）
	Breakdown          string    `gorm:"type:jsonb;default:'{}'"`                                   // 各維度細節（JSON 字串）
	UpdatedAt          time.Time `json:"updated_at"`
}

func (StockChipScore) TableName() string { return "stock_chip_scores" }

// ChipScoreJob 記錄每次籌碼評分批次計算作業狀態
type ChipScoreJob struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `gorm:"default:running"         json:"status"` // running|completed|failed
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Fail        int        `json:"fail"`
	Message     string     `json:"message"`
}

func (ChipScoreJob) TableName() string { return "chip_score_jobs" }
