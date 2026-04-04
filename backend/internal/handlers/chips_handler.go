package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	chips "stock-backend/internal/chips"
	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChipsHandler struct {
	db         *gorm.DB
	runner     *chips.Runner
	scraperURL string // Python scraper 服務 URL
}

var (
	chipsRunnerOnce sync.Once
	chipsRunner     *chips.Runner
)

func getChipsRunner(db *gorm.DB) *chips.Runner {
	chipsRunnerOnce.Do(func() {
		chipsRunner = chips.NewRunner(db)
		if err := chipsRunner.RecoverStaleJobs(); err != nil {
			log.Printf("[chips] recover stale jobs failed: %v", err)
		}
	})
	return chipsRunner
}

func NewChipsHandler(db *gorm.DB) *ChipsHandler {
	return &ChipsHandler{
		db:         db,
		runner:     getChipsRunner(db),
		scraperURL: os.Getenv("SCRAPER_URL"),
	}
}

// ─── 方案派送 ─────────────────────────────────────────────────────────────────

// callPythonScraper 呼叫 Python scraper 服務
func (h *ChipsHandler) callPythonScraper(method string, symbol string) error {
	if h.scraperURL == "" {
		return fmt.Errorf("SCRAPER_URL 未設定，無法呼叫 Python scraper 服務")
	}
	body := map[string]string{"method": method}
	if symbol != "" {
		body["symbol"] = symbol
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, h.scraperURL+"/trigger", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	chips.WriteLog(h.db, nil, "info", "python_scraper", "",
		fmt.Sprintf("呼叫 Python scraper: url=%s method=%s symbol=%s", h.scraperURL+"/trigger", method, symbol))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("python scraper 無法連線: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return chips.ErrJobRunning
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("python scraper 回傳 %d: %s", resp.StatusCode, string(b))
		chips.WriteLog(h.db, nil, "error", "python_scraper", symbol, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// triggerByScheme 依方案觸發爬取
func (h *ChipsHandler) triggerByScheme(scheme string, symbol string) (int, error) {
	switch scheme {
	case "python_http":
		return 0, h.callPythonScraper("http", symbol)
	case "python_playwright":
		return 0, h.callPythonScraper("playwright", symbol)
	default: // go_http
		return h.runner.TriggerWithScheme(symbol, scheme)
	}
}

// dispatchTrigger 讀取設定完成方案派送，失敗時自動啟用備案
func (h *ChipsHandler) dispatchTrigger(symbol string) (scheme string, total int, err error) {
	cfg := GetFeatureConfig(h.db, "chips_pyramid")
	scheme = cfg.Primary
	chips.WriteLog(h.db, nil, "info", "dispatch", symbol,
		fmt.Sprintf("使用方案=%s fallback_enabled=%v fallback=%s", scheme, cfg.FallbackEnabled, cfg.Fallback))
	total, err = h.triggerByScheme(scheme, symbol)
	if err != nil && err != chips.ErrJobRunning && err != chips.ErrNoSymbols {
		chips.WriteLog(h.db, nil, "error", "dispatch", symbol,
			fmt.Sprintf("主方案 %s 失敗: %v", scheme, err))
		if cfg.FallbackEnabled && cfg.Fallback != "" && cfg.Fallback != scheme {
			log.Printf("[chips] 主方案 %s 失敗（%v），嘗試備案 %s", scheme, err, cfg.Fallback)
			scheme = cfg.Fallback
			chips.WriteLog(h.db, nil, "warn", "dispatch", symbol,
				fmt.Sprintf("啟用備案方案=%s", scheme))
			total, err = h.triggerByScheme(scheme, symbol)
			if err != nil {
				chips.WriteLog(h.db, nil, "error", "dispatch", symbol,
					fmt.Sprintf("備案方案 %s 也失敗: %v", scheme, err))
			}
		}
	}
	return
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
		"scheme":       job.Scheme,
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
// 手動觸發一次爬取（依設定選擇方案）
func (h *ChipsHandler) Trigger(c *gin.Context) {
	scheme, total, err := h.dispatchTrigger("")
	if err != nil {
		if err == chips.ErrJobRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "scraper already running"})
			return
		}
		if err == chips.ErrNoSymbols {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total, "scheme": scheme})
}

// TriggerSingle POST /api/chips/trigger-single
// body: {"symbol":"2330"}
// 觸發單支股票筌碼爬取，可用 /api/chips/status 追蹤進度
func (h *ChipsHandler) TriggerSingle(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求格式錯誤"})
		return
	}
	symbol := strings.TrimSpace(strings.ToUpper(body.Symbol))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需提供股票代號"})
		return
	}

	scheme, total, err := h.dispatchTrigger(symbol)
	if err != nil {
		if err == chips.ErrJobRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "已有爬取任務執行中"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "symbol": symbol, "total": total, "scheme": scheme})
}

// Cancel POST /api/chips/cancel
// 取消目前執行中的籌碼爬取 job。
// 1. 先嘗試透過 Go runner 的 context 優雅取消（適用新版代碼）
// 2. 同時強制將 DB 中最新的 running job 標為 cancelled（適用舊版 / 任何卡住的 job）
func (h *ChipsHandler) Cancel(c *gin.Context) {
	cancelledInRunner := h.runner.Cancel()

	// 強制將 DB 中處於 running 的最新 job 標為 cancelled
	now := time.Now()
	result := h.db.Model(&models.ChipsSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "cancelled",
			"completed_at": &now,
			"message":      "使用者手動取消",
		})

	dbCancelled := result.RowsAffected > 0

	if !cancelledInRunner && !dbCancelled {
		c.JSON(http.StatusConflict, gin.H{"error": "沒有執行中的 job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                true,
		"runner_cancelled":  cancelledInRunner,
		"db_rows_cancelled": result.RowsAffected,
		"message":           "已取消",
	})
}

// TriggerCron 由後端 cron goroutine 呼叫（不走 HTTP）
func TriggerCron(db *gorm.DB) {
	runner := getChipsRunner(db)
	total, err := runner.Trigger("")
	if err != nil {
		if err == chips.ErrJobRunning {
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

// GetLogs GET /api/chips/logs
// Query params:
//   - limit  int     預設 100，最多 500
//   - level  string  過濾：info | warn | error
//   - job_id uint    對应 job_id
//   - symbol string  對应股票代號
func (h *ChipsHandler) GetLogs(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	query := h.db.Model(&models.ChipsRunLog{}).Order("id DESC").Limit(limit)

	if level := c.Query("level"); level != "" {
		query = query.Where("level = ?", level)
	}
	if jobID := c.Query("job_id"); jobID != "" {
		query = query.Where("job_id = ?", jobID)
	}
	if symbol := c.Query("symbol"); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	var logs []models.ChipsRunLog
	if err := query.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "count": len(logs)})
}

// GetSymbolLatest GET /api/chips/:symbol/latest
// 回傳指定股票的最新持股分佈快照（含分層明細），無資料時回傳 data: null。
func (h *ChipsHandler) GetSymbolLatest(c *gin.Context) {
	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需提供股票代號"})
		return
	}

	var snapshot models.ChipsHolderSnapshot
	result := h.db.
		Preload("Distributions", func(db *gorm.DB) *gorm.DB {
			return db.Order("tier_rank ASC")
		}).
		Where("symbol = ?", symbol).
		Order("data_date DESC").
		First(&snapshot)

	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": snapshot})
}
