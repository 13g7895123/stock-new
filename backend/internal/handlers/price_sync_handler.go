package handlers

import (
	"log"
	"net/http"
	"sync"

	"stock-backend/internal/models"
	pricesrunner "stock-backend/internal/prices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PriceSyncHandler struct {
	db     *gorm.DB
	runner *pricesrunner.Runner
}

var (
	priceSyncRunnerOnce sync.Once
	priceSyncRunner     *pricesrunner.Runner
)

func getPriceSyncRunner(db *gorm.DB) *pricesrunner.Runner {
	priceSyncRunnerOnce.Do(func() {
		priceSyncRunner = pricesrunner.NewRunner(db)
		if err := priceSyncRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[price-sync] recover stale jobs failed: %v", err)
		}
	})
	return priceSyncRunner
}

func NewPriceSyncHandler(db *gorm.DB) *PriceSyncHandler {
	return &PriceSyncHandler{db: db, runner: getPriceSyncRunner(db)}
}

// Status GET /api/scraper/prices/all/status
// 回傳最近一次全股票歷史日K批次作業狀態
func (h *PriceSyncHandler) Status(c *gin.Context) {
	var job models.PriceSyncJob
	result := h.db.Order("id DESC").First(&job)
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "never",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           job.ID,
		"status":       job.Status,
		"started_at":   job.StartedAt,
		"completed_at": job.CompletedAt,
		"total":        job.Total,
		"success":      job.Success,
		"fail":         job.Fail,
		"message":      job.Message,
	})
}

// Trigger POST /api/scraper/prices/all/trigger
// 手動觸發一次全股票歷史日K批次爬取
func (h *PriceSyncHandler) Trigger(c *gin.Context) {
	total, err := h.runner.Trigger()
	if err != nil {
		if err == pricesrunner.ErrJobRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
			return
		}
		if err == pricesrunner.ErrNoSymbols {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total})
}
