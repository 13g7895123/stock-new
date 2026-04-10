package handlers

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	institutionalrunner "stock-backend/internal/institutional"
	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InstitutionalHandler struct {
	db     *gorm.DB
	runner *institutionalrunner.Runner
}

var (
	institutionalRunnerOnce sync.Once
	institutionalRunner     *institutionalrunner.Runner
)

func getInstitutionalRunner(db *gorm.DB) *institutionalrunner.Runner {
	institutionalRunnerOnce.Do(func() {
		institutionalRunner = institutionalrunner.NewRunner(db)
		if err := institutionalRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[institutional] recover stale jobs failed: %v", err)
		}
	})
	return institutionalRunner
}

func NewInstitutionalHandler(db *gorm.DB) *InstitutionalHandler {
	return &InstitutionalHandler{db: db, runner: getInstitutionalRunner(db)}
}

// Status GET /api/institutional/status
func (h *InstitutionalHandler) Status(c *gin.Context) {
	var job models.InstitutionalSyncJob
	if err := h.db.Order("id DESC").First(&job).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "never"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// Trigger POST /api/institutional/trigger
// body: {"days": 1}
func (h *InstitutionalHandler) Trigger(c *gin.Context) {
	var body struct {
		Days int `json:"days"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Days <= 0 {
		body.Days = 1
	}

	total, err := h.runner.Trigger(body.Days)
	if err != nil {
		switch err {
		case institutionalrunner.ErrJobRunning:
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total, "days": body.Days})
}

// GetBySymbol GET /api/institutional/:symbol?limit=30
// 回傳指定股票最近幾日的三大法人資料（由新到舊）
func (h *InstitutionalHandler) GetBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	limitStr := c.DefaultQuery("limit", "30")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 365 {
		limit = 30
	}

	var records []models.InstitutionalTrading
	if err := h.db.
		Where("symbol = ?", symbol).
		Order("date DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":  symbol,
		"limit":   limit,
		"records": records,
	})
}

// TriggerInstitutionalCron 由排程器呼叫
func TriggerInstitutionalCron(db *gorm.DB, days int) error {
	runner := getInstitutionalRunner(db)
	if days <= 0 {
		days = 1
	}
	_, err := runner.Trigger(days)
	if err == institutionalrunner.ErrJobRunning {
		log.Printf("[institutional-cron] 已有作業執行中，略過本次排程 days=%d", days)
		return nil
	}
	return err
}
