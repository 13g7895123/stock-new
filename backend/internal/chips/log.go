package chips

import (
	"log"

	"stock-backend/internal/models"

	"gorm.io/gorm"
)

// WriteLog 寫入一筆 chips 執行日誌到 DB。
// jobID 為 nil 時表示尚無 job（handler 層觸發前的日誌）。
func WriteLog(db *gorm.DB, jobID *uint, level, step, symbol, message string) {
	entry := models.ChipsRunLog{
		JobID:   jobID,
		Level:   level,
		Step:    step,
		Symbol:  symbol,
		Message: message,
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("[chips-log] db write failed: %v", err)
	}
}

// jobPtr 將 uint 轉為 *uint（方便傳入 jobID）
func jobPtr(id uint) *uint { return &id }
