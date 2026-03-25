package handlers

import (
	"log"
	"net/http"
	"sync"
	"time"

	chipsrunner "stock-backend/internal/chips"
	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChipsHandler struct {
	db     *gorm.DB
	runner *chipsrunner.Runner
}

var (
	chipsRunnerOnce sync.Once
	chipsRunner     *chipsrunner.Runner
)

func getChipsRunner(db *gorm.DB) *chipsrunner.Runner {
	chipsRunnerOnce.Do(func() {
		chipsRunner = chipsrunner.NewRunner(db)
		if err := chipsRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[chips] recover stale jobs failed: %v", err)
		}
	})
	return chipsRunner
}

func NewChipsHandler(db *gorm.DB) *ChipsHandler {
	return &ChipsHandler{db: db, runner: getChipsRunner(db)}
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
			"next_run": nextSunday().Format(time.RFC3339),
		})
		return
	}

	// "fresh" = 本週日之後有成功完成的 job
	lastSun := lastSunday()
	isFresh := job.Status == "completed" && job.CompletedAt != nil && job.CompletedAt.After(lastSun)

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
		"next_run":     nextSunday().Format(time.RFC3339),
	})
}

// Trigger POST /api/chips/trigger
// 手動觸發一次爬取（由 Go backend 背景執行）
func (h *ChipsHandler) Trigger(c *gin.Context) {
	total, err := h.runner.Trigger("")
	if err != nil {
		if err == chipsrunner.ErrJobRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "scraper already running"})
			return
		}
		if err == chipsrunner.ErrNoSymbols {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total})
}

// TriggerCron 由後端 cron goroutine 呼叫（不走 HTTP）
func TriggerCron(db *gorm.DB) {
	runner := getChipsRunner(db)
	total, err := runner.Trigger("")
	if err != nil {
		if err == chipsrunner.ErrJobRunning {
			log.Printf("[chips-cron] 已有籌碼作業執行中，略過本次排程")
			return
		}
		log.Printf("[chips-cron] 觸發失敗: %v", err)
		return
	}
	log.Printf("[chips-cron] 已觸發 Go 籌碼爬取，共 %d 檔", total)
}

// lastSunday 回傳上一個（或本）週日零時（Asia/Taipei）
func lastSunday() time.Time {
	loc, _ := time.LoadLocation("Asia/Taipei")
	now := time.Now().In(loc)
	// Sunday = 0，daysSince = Weekday() % 7（Sunday=0 → 0 days ago）
	daysSinceSun := int(now.Weekday())
	last := now.AddDate(0, 0, -daysSinceSun)
	return time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)
}

// nextSunday 回傳下一個週日 10:00（Asia/Taipei）
func nextSunday() time.Time {
	loc, _ := time.LoadLocation("Asia/Taipei")
	now := time.Now().In(loc)
	daysUntilSun := (7 - int(now.Weekday())) % 7
	if daysUntilSun == 0 {
		daysUntilSun = 7
	}
	next := now.AddDate(0, 0, daysUntilSun)
	return time.Date(next.Year(), next.Month(), next.Day(), 10, 0, 0, 0, loc)
}
