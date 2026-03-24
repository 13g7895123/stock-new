package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChipsHandler struct {
	db *gorm.DB
}

func NewChipsHandler(db *gorm.DB) *ChipsHandler {
	return &ChipsHandler{db: db}
}

// Status GET /api/chips/status
// 回傳最近一次籌碼爬取 job 的狀態，並計算是否為本週最新。
func (h *ChipsHandler) Status(c *gin.Context) {
	var job models.ChipsSyncJob
	result := h.db.Order("id DESC").First(&job)
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":   "never",
			"is_fresh": false,
			"next_run": nextSaturday().Format(time.RFC3339),
		})
		return
	}

	// "fresh" = 本週六之後有成功完成的 job
	lastSat := lastSaturday()
	isFresh := job.Status == "completed" && job.StartedAt.After(lastSat)

	c.JSON(http.StatusOK, gin.H{
		"id":           job.ID,
		"status":       job.Status,
		"started_at":   job.StartedAt,
		"completed_at": job.CompletedAt,
		"total":        job.Total,
		"success":      job.Success,
		"fail":         job.Fail,
		"message":      job.Message,
		"is_fresh":     isFresh,
		"next_run":     nextSaturday().Format(time.RFC3339),
	})
}

// Trigger POST /api/chips/trigger
// 手動觸發一次爬取（呼叫 scraper HTTP service）
func (h *ChipsHandler) Trigger(c *gin.Context) {
	scraperURL := scraperBaseURL() + "/trigger"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scraperURL, bytes.NewBufferString("{}"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("scraper unavailable: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		c.JSON(http.StatusConflict, gin.H{"error": "scraper already running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TriggerCron 由後端 cron goroutine 呼叫（不走 HTTP）
func TriggerCron(db *gorm.DB) {
	scraperURL := scraperBaseURL() + "/trigger"

	log.Printf("[chips-cron] 週六自動觸發籌碼爬取 → %s", scraperURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scraperURL,
		bytes.NewBufferString("{}"))
	if err != nil {
		log.Printf("[chips-cron] 建立 request 失敗: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[chips-cron] 呼叫 scraper 失敗: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[chips-cron] scraper 回應 %d", resp.StatusCode)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scraperBaseURL() string {
	if u := os.Getenv("CHIPS_SCRAPER_URL"); u != "" {
		return u
	}
	return "http://chips_scraper:5100"
}

// lastSaturday 回傳上一個（或本）週六零時（Asia/Taipei）
func lastSaturday() time.Time {
	loc, _ := time.LoadLocation("Asia/Taipei")
	now := time.Now().In(loc)
	daysSinceSat := int(now.Weekday()+1) % 7 // 週六 = 6 → 0 days ago
	last := now.AddDate(0, 0, -daysSinceSat)
	return time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)
}

// nextSaturday 回傳下一個週六零時
func nextSaturday() time.Time {
	loc, _ := time.LoadLocation("Asia/Taipei")
	now := time.Now().In(loc)
	daysUntilSat := (6 - int(now.Weekday()) + 7) % 7
	if daysUntilSat == 0 {
		daysUntilSat = 7
	}
	next := now.AddDate(0, 0, daysUntilSat)
	return time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, loc)
}
