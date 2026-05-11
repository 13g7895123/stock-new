package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"stock-backend/internal/models"
	statusrunner "stock-backend/internal/stockstatus"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StockStatusHandler struct {
	db     *gorm.DB
	runner *statusrunner.Runner
}

var (
	stockStatusRunnerOnce sync.Once
	stockStatusRunner     *statusrunner.Runner
)

func getStockStatusRunner(db *gorm.DB) *statusrunner.Runner {
	stockStatusRunnerOnce.Do(func() {
		stockStatusRunner = statusrunner.NewRunner(db)
		if err := stockStatusRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[stock-status-sync] recover stale jobs failed: %v", err)
		}
	})
	return stockStatusRunner
}

func NewStockStatusHandler(db *gorm.DB) *StockStatusHandler {
	return &StockStatusHandler{db: db, runner: getStockStatusRunner(db)}
}

func (h *StockStatusHandler) Status(c *gin.Context) {
	var job models.StockStatusSyncJob
	if err := h.db.Order("id DESC").First(&job).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "never", "running": h.runner.IsRunning()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           job.ID,
		"status":       job.Status,
		"running":      h.runner.IsRunning(),
		"started_at":   job.StartedAt,
		"completed_at": job.CompletedAt,
		"total":        job.Total,
		"message":      job.Message,
	})
}

func (h *StockStatusHandler) Sync(c *gin.Context) {
	result, err := h.runner.Trigger()
	if err != nil {
		if errors.Is(err, statusrunner.ErrJobRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": "已有狀態同步作業執行中"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
}

func (h *StockStatusHandler) List(c *gin.Context) {
	asOf, err := parseStatusAsOf(c.Query("as_of"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
		return
	}
	market := strings.ToUpper(strings.TrimSpace(c.Query("market")))
	if market != "" && market != "TWSE" && market != "TPEX" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "market must be TWSE or TPEX"})
		return
	}
	statusType := strings.TrimSpace(c.Query("type"))
	if statusType != "" && statusType != models.StockStatusDisposition && statusType != models.StockStatusAttention && statusType != models.StockStatusDayTradeRestricted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be disposition, attention, or day_trade_restricted"})
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))

	q := h.db.Model(&models.StockStatus{}).
		Where("start_date <= ? AND end_date >= ?", asOf, asOf).
		Order("symbol ASC, type ASC, start_date ASC")
	if market != "" {
		q = q.Where("market = ?", market)
	}
	if statusType != "" {
		q = q.Where("type = ?", statusType)
	}
	if symbol != "" {
		q = q.Where("symbol = ?", symbol)
	}

	var statuses []models.StockStatus
	if err := q.Find(&statuses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"as_of": asOf.Format("2006-01-02"),
		"count": len(statuses),
		"data":  statuses,
	})
}

func TriggerStockStatusCron(db *gorm.DB) error {
	runner := getStockStatusRunner(db)
	_, err := runner.Trigger()
	if errors.Is(err, statusrunner.ErrJobRunning) {
		log.Printf("[stock-status-cron] 已有作業執行中，略過本次排程")
		return nil
	}
	return err
}

func parseStatusAsOf(raw string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.Local
	}
	if raw == "" {
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
}
