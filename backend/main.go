package main

import (
	"log"

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

	// ── 動態排程器：由 task_schedules 資料表驅動，每分鐘檢查並觸發到期任務 ──
	go handlers.RunScheduler(db)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
