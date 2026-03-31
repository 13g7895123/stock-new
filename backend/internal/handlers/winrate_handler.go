package handlers

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"stock-backend/internal/models"
	winraterunner "stock-backend/internal/winrate"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WinrateHandler struct {
	db     *gorm.DB
	runner *winraterunner.Runner
}

var (
	winrateRunnerOnce sync.Once
	winrateRunnerInst *winraterunner.Runner
)

func getWinrateRunner(db *gorm.DB) *winraterunner.Runner {
	winrateRunnerOnce.Do(func() {
		winrateRunnerInst = winraterunner.NewRunner(db)
		if err := winrateRunnerInst.RecoverStaleJobs(); err != nil {
			log.Printf("[winrate] recover stale jobs failed: %v", err)
		}
	})
	return winrateRunnerInst
}

func NewWinrateHandler(db *gorm.DB) *WinrateHandler {
	return &WinrateHandler{db: db, runner: getWinrateRunner(db)}
}

// Status  GET /api/winrate/status
func (h *WinrateHandler) Status(c *gin.Context) {
	var job models.WinrateSyncJob
	if err := h.db.Order("id DESC").First(&job).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "never"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// Trigger  POST /api/winrate/trigger  — 計算全部有主力進出資料的股票
func (h *WinrateHandler) Trigger(c *gin.Context) {
	total, err := h.runner.Trigger("")
	if err != nil {
		switch err {
		case winraterunner.ErrJobRunning:
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total})
}

// TriggerSingle  POST /api/winrate/trigger-single  body: {"symbol":"2330"}
func (h *WinrateHandler) TriggerSingle(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	total, err := h.runner.Trigger(body.Symbol)
	if err != nil {
		switch err {
		case winraterunner.ErrJobRunning:
			c.JSON(http.StatusConflict, gin.H{"error": "已有作業執行中"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total})
}

// GetWinrateBySymbol  GET /api/stocks/:symbol/broker-winrate?min_entries=2
// 回傳該股票所有券商的勝率摘要，按勝率降冪排列
func (h *WinrateHandler) GetWinrateBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	minEntries := 2
	if v := c.Query("min_entries"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minEntries = n
		}
	}

	type Row struct {
		BrokerName     string   `json:"broker_name"      gorm:"column:broker_name"`
		TotalEntries   int      `json:"total_entries"    gorm:"column:total_entries"`
		TotalExits     int      `json:"total_exits"      gorm:"column:total_exits"`
		WinCount       int      `json:"win_count"        gorm:"column:win_count"`
		WinRatePct     *float64 `json:"win_rate_pct"     gorm:"column:win_rate_pct"`
		AvgReturnPct   *float64 `json:"avg_return_pct"   gorm:"column:avg_return_pct"`
		AvgHoldingDays *float64 `json:"avg_holding_days" gorm:"column:avg_holding_days"`
		MaxReturnPct   *float64 `json:"max_return_pct"   gorm:"column:max_return_pct"`
		LastEntryDate  *string  `json:"last_entry_date"  gorm:"column:last_entry_date"`
	}

	var rows []Row
	if err := h.db.Raw(`
		SELECT
			broker_name,
			COUNT(*)                                       AS total_entries,
			COUNT(exit_date)                               AS total_exits,
			COUNT(CASE WHEN is_win THEN 1 END)             AS win_count,
			ROUND(
				COUNT(CASE WHEN is_win THEN 1 END)::numeric
				/ NULLIF(COUNT(exit_date), 0) * 100, 2
			)                                              AS win_rate_pct,
			ROUND(AVG(return_pct)   FILTER (WHERE exit_date IS NOT NULL), 2) AS avg_return_pct,
			ROUND(AVG(holding_days) FILTER (WHERE exit_date IS NOT NULL), 1) AS avg_holding_days,
			ROUND(MAX(return_pct), 2)                      AS max_return_pct,
			TO_CHAR(MAX(entry_date), 'YYYY-MM-DD')         AS last_entry_date
		FROM broker_trade_events
		WHERE symbol = ?
		GROUP BY broker_name
		HAVING COUNT(*) >= ?
		ORDER BY win_rate_pct DESC NULLS LAST, total_entries DESC
	`, symbol, minEntries).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// GetEventsByBroker  GET /api/stocks/:symbol/broker-winrate/events?broker=元大-中山
// 回傳指定股票＋券商的所有歷史交易事件（建倉→出場明細）
func (h *WinrateHandler) GetEventsByBroker(c *gin.Context) {
	symbol := c.Param("symbol")
	brokerName := c.Query("broker")
	if brokerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "broker query param is required"})
		return
	}

	var events []models.BrokerTradeEvent
	if err := h.db.Where("symbol = ? AND broker_name = ?", symbol, brokerName).
		Order("entry_date DESC").
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}
