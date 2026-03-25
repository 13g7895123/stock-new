package main

import (
	"log"
	"time"

	"stock-backend/internal/config"
	"stock-backend/internal/database"
	"stock-backend/internal/handlers"
	"stock-backend/internal/routes"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	r := routes.Setup(db)

	// ── 籌碼金字塔週排程：每週日 10:00 自動觸發爬取 ──
	// （網站通常週六更新，週日 10:00 確保資料已發布）
	go func() {
		for {
			now := time.Now()
			// 下一個週日 10:00 (Asia/Taipei)
			loc, _ := time.LoadLocation("Asia/Taipei")
			nowTaipei := now.In(loc)
			daysUntilSun := (7 - int(nowTaipei.Weekday())) % 7
			if daysUntilSun == 0 {
				daysUntilSun = 7
			}
			nextSun := time.Date(
				nowTaipei.Year(), nowTaipei.Month(), nowTaipei.Day()+daysUntilSun,
				10, 0, 0, 0, loc,
			)
			sleep := nextSun.Sub(now)
			log.Printf("[chips-cron] 下次自動爬取於 %s（%.1f 小時後）", nextSun.Format("2006-01-02 15:04"), sleep.Hours())
			time.Sleep(sleep)
			handlers.TriggerCron(db)
		}
	}()

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
