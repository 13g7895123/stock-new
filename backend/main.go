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

	// ── 籌碼金字塔週排程：每週六 08:00 自動觸發爬取 ──
	go func() {
		for {
			now := time.Now()
			// 下一個週六 08:00 (Asia/Taipei)
			loc, _ := time.LoadLocation("Asia/Taipei")
			nowTaipei := now.In(loc)
			daysUntilSat := (6 - int(nowTaipei.Weekday()) + 7) % 7
			if daysUntilSat == 0 {
				daysUntilSat = 7
			}
			nextSat := time.Date(
				nowTaipei.Year(), nowTaipei.Month(), nowTaipei.Day()+daysUntilSat,
				8, 0, 0, 0, loc,
			)
			sleep := nextSat.Sub(now)
			log.Printf("[chips-cron] 下次自動爬取於 %s（%.1f 小時後）", nextSat.Format("2006-01-02 15:04"), sleep.Hours())
			time.Sleep(sleep)
			handlers.TriggerCron(db)
		}
	}()

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
