package handlers

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	majorrunner "stock-backend/internal/major"
	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MajorHandler struct {
	db     *gorm.DB
	runner *majorrunner.Runner
}

var (
	majorRunnerOnce sync.Once
	majorRunner     *majorrunner.Runner
)

func getMajorRunner(db *gorm.DB) *majorrunner.Runner {
	majorRunnerOnce.Do(func() {
		majorRunner = majorrunner.NewRunner(db)
		if err := majorRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[major] recover stale jobs failed: %v", err)
		}
	})
	return majorRunner
}

func NewMajorHandler(db *gorm.DB) *MajorHandler {
	return &MajorHandler{db: db, runner: getMajorRunner(db)}
}

// Status GET /api/major/status
// 回傳最近一次主力進出批次爬取作業狀態
func (h *MajorHandler) Status(c *gin.Context) {
	var job models.MajorSyncJob
	if err := h.db.Order("id DESC").First(&job).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "never"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// Trigger POST /api/major/trigger
// body: {"days": 1}  （days 可省略，預設 1）
func (h *MajorHandler) Trigger(c *gin.Context) {
	var body struct {
		Days int `json:"days"`
	}
	// BindJSON 失敗時使用預設值 1，不回傳錯誤
	_ = c.ShouldBindJSON(&body)
	if body.Days <= 0 {
		body.Days = 1
	}

	total, err := h.runner.Trigger("", body.Days)
	if err != nil {
		switch err {
		case majorrunner.ErrJobRunning:
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
		case majorrunner.ErrNoSymbols:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total, "days": body.Days})
}

// TriggerSingle POST /api/major/trigger-single
// body: {"symbol":"2330","days":1}
// 非同步爬取單支股票（加入排程，非即時回傳資料）
func (h *MajorHandler) TriggerSingle(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol"`
		Days   int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請提供 symbol 欄位"})
		return
	}
	if body.Days <= 0 {
		body.Days = 1
	}

	total, err := h.runner.Trigger(body.Symbol, body.Days)
	if err != nil {
		switch err {
		case majorrunner.ErrJobRunning:
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total, "symbol": body.Symbol, "days": body.Days})
}

// TestSingle POST /api/major/test
// body: {"symbol":"2330","days":1}
// 同步爬取單支股票並直接回傳結果，用於 API 診斷
func (h *MajorHandler) TestSingle(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol"`
		Days   int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請提供 symbol 欄位"})
		return
	}
	if body.Days <= 0 {
		body.Days = 1
	}

	result, err := h.runner.FetchSingle(body.Symbol, body.Days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":     false,
			"symbol": body.Symbol,
			"days":   body.Days,
			"tried":  result.Tried,
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"symbol":    body.Symbol,
		"days":      body.Days,
		"source":    result.Source,
		"url":       result.URL,
		"data_date": result.DataDate.Format("2006-01-02"),
		"count":     len(result.Records),
		"records":   result.Records,
	})
}

// GetBySymbol GET /api/major/:symbol?days=1
// 回傳指定股票最新資料日的主力進出（買超 / 賣超 分組）
func (h *MajorHandler) GetBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	daysStr := c.DefaultQuery("days", "1")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 1
	}

	// 查詢該股票 + days 最新的 data_date
	var latestDate time.Time
	if err := h.db.Model(&models.MajorBrokerRecord{}).
		Where("symbol = ? AND days = ?", symbol, days).
		Select("MAX(data_date)").
		Scan(&latestDate).Error; err != nil || latestDate.IsZero() {
		c.JSON(http.StatusOK, gin.H{
			"symbol":    symbol,
			"days":      days,
			"data_date": nil,
			"buy":       []any{},
			"sell":      []any{},
		})
		return
	}

	var records []models.MajorBrokerRecord
	if err := h.db.
		Where("symbol = ? AND days = ? AND data_date = ?", symbol, days, latestDate).
		Order("side ASC, rank ASC").
		Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	buy := make([]models.MajorBrokerRecord, 0)
	sell := make([]models.MajorBrokerRecord, 0)
	for _, r := range records {
		if r.Side == "buy" {
			buy = append(buy, r)
		} else {
			sell = append(sell, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"days":      days,
		"data_date": latestDate.Format("2006-01-02"),
		"buy":       buy,
		"sell":      sell,
	})
}
