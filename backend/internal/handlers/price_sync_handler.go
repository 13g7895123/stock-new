package handlers

import (
	"log"
	"net/http"
	"regexp"
	"strings"
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

var validSymbolRe = regexp.MustCompile(`^[1-9][0-9]{3}$`)

// TestSingle POST /api/scraper/prices/all/test
// body: {"symbol":"2330"}
// 同步爬取單支股票全部歷史日K，直接回傳結果（用於測試診斷）
func (h *PriceSyncHandler) TestSingle(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求格式錯誤"})
		return
	}
	symbol := strings.TrimSpace(strings.ToUpper(body.Symbol))
	if !validSymbolRe.MatchString(symbol) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "股票代號格式不正確（需為四碼非零開頭，如 2330）"})
		return
	}

	var stock models.Stock
	if err := h.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "找不到此股票代號，請先執行同步股票清單"})
		return
	}

	log.Printf("[price-sync] test single symbol=%s market=%s", stock.Symbol, stock.Market)
	n, err := h.runner.FetchSingle(stock.Symbol, stock.Market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":     false,
			"symbol": stock.Symbol,
			"market": stock.Market,
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"symbol":  stock.Symbol,
		"market":  stock.Market,
		"records": n,
	})
}

// TriggerPriceCron 由排程器呼叫，觸發全量日K爬取（非 HTTP）
func TriggerPriceCron(db *gorm.DB) error {
	runner := getPriceSyncRunner(db)
	_, err := runner.Trigger()
	if err == pricesrunner.ErrJobRunning {
		log.Printf("[price-cron] 已有作業執行中，略過本次排程")
		return nil
	}
	return err
}
