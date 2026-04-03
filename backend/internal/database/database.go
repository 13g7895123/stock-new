package database

import (
	"fmt"

	"stock-backend/internal/config"
	"stock-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Taipei",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// AutoMigrate：自動建表 / 新增欄位 / 新增 index（不刪欄位、不改型態）
	if err := db.AutoMigrate(
		&models.Stock{},
		&models.Tag{},
		&models.StockGroup{},
		&models.DailyPrice{},
		&models.ChipsSyncJob{},
		&models.ChipsHolderSnapshot{},
		&models.ChipsHolderDistribution{},
		&models.ChipsRunLog{},
		&models.PriceSyncJob{},
		&models.AppSetting{},
		&models.MajorSyncJob{},
		&models.MajorBrokerRecord{},
		&models.BrokerTradeEvent{},
		&models.WinrateSyncJob{},
		&models.TaskSchedule{},
	); err != nil {
		return nil, err
	}

	// 手動遷移：處理 AutoMigrate 無法完成的變更（改型態、刪欄位、重命名等）
	// 每條 SQL 必須可冪等重複執行（IF EXISTS / IF NOT EXISTS / DO NOTHING 等）
	if err := runManualMigrations(db); err != nil {
		return nil, err
	}

	return db, nil
}

// runManualMigrations 執行無法由 AutoMigrate 自動處理的 schema 變更。
// 規則：
//  1. 每條 SQL 必須冪等（重複執行結果相同，不報錯）
//  2. 加新條目時在最下方附上日期註解，方便追蹤
//  3. 已確認線上環境套用後的舊條目可保留（反正冪等），也可移除以保持整潔
func runManualMigrations(db *gorm.DB) error {
	stmts := []string{
		// ── 範例（依需求取消註解或新增）──────────────────────────────────────
		// 2026-03-25  刪除舊欄位
		// `ALTER TABLE stocks DROP COLUMN IF EXISTS old_column`,
		//
		// 2026-03-25  欄位改型態（需先確認資料可轉換）
		// `ALTER TABLE daily_prices ALTER COLUMN volume TYPE bigint USING volume::bigint`,
		//
		// 2026-03-25  重命名欄位（PostgreSQL 不支援 IF NOT EXISTS，用 DO $$ 包裝）
		// `DO $$ BEGIN
		//    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='stocks' AND column_name='old_name') THEN
		//      ALTER TABLE stocks RENAME COLUMN old_name TO new_name;
		//    END IF;
		//  END $$`,
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("manual migration failed [%s]: %w", s[:min(40, len(s))], err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
