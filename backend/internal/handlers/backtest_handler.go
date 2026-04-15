package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"stock-backend/internal/backtest"
	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BacktestHandler struct {
	db      *gorm.DB
	mu      sync.Mutex
	running map[uint]bool
}

func NewBacktestHandler(db *gorm.DB) *BacktestHandler {
	return &BacktestHandler{db: db, running: map[uint]bool{}}
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/backtest/jobs
// ────────────────────────────────────────────────────────────────────────────
func (h *BacktestHandler) ListJobs(c *gin.Context) {
	var jobs []models.BacktestJob
	h.db.Order("started_at DESC").Limit(50).Find(&jobs)
	c.JSON(http.StatusOK, jobs)
}

// ────────────────────────────────────────────────────────────────────────────
// POST /api/backtest/run
// Body JSON: { symbol, start_date, end_date, capital, params:{...} }
// ────────────────────────────────────────────────────────────────────────────
func (h *BacktestHandler) Run(c *gin.Context) {
	type reqBody struct {
		Symbol    string                  `json:"symbol"`
		StartDate string                  `json:"start_date"`
		EndDate   string                  `json:"end_date"`
		Capital   float64                 `json:"capital"`
		Params    backtest.StrategyParams `json:"params"`
	}

	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求格式錯誤"})
		return
	}
	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請指定股票代碼"})
		return
	}
	if req.StartDate == "" {
		req.StartDate = time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	}
	if req.EndDate == "" {
		req.EndDate = time.Now().Format("2006-01-02")
	}
	if req.Capital <= 0 {
		req.Capital = 1_000_000
	}

	// 填補預設參數
	def := backtest.DefaultParams()
	if req.Params.EntryMAShort == 0 { req.Params.EntryMAShort = def.EntryMAShort }
	if req.Params.EntryMALong  == 0 { req.Params.EntryMALong  = def.EntryMALong }
	if req.Params.ExitMAShort  == 0 { req.Params.ExitMAShort  = def.ExitMAShort }
	if req.Params.ExitMALong   == 0 { req.Params.ExitMALong   = def.ExitMALong }
	if req.Params.CapitalPerTrade == 0 { req.Params.CapitalPerTrade = def.CapitalPerTrade }
	if req.Params.MaxPositions == 0 { req.Params.MaxPositions = def.MaxPositions }
	if req.Params.StopLossPct  == 0 { req.Params.StopLossPct  = def.StopLossPct }
	if req.Params.TakeProfitPct == 0 { req.Params.TakeProfitPct = def.TakeProfitPct }
	if req.Params.MaxHoldDays  == 0 { req.Params.MaxHoldDays  = def.MaxHoldDays }
	if req.Params.FeeRate  == 0 { req.Params.FeeRate  = def.FeeRate }
	if req.Params.TaxRate   == 0 { req.Params.TaxRate   = def.TaxRate }

	paramsJSON, _ := json.Marshal(req.Params)

	// 建立 job
	job := models.BacktestJob{
		Symbol:    req.Symbol,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Capital:   req.Capital,
		Params:    string(paramsJSON),
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := h.db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步執行（單股票速度快，通常 <1s）
	res := backtest.Run(h.db, job.ID, req.Symbol, req.StartDate, req.EndDate, req.Capital, req.Params)

	// 儲存結果
	now := time.Now()
	job.Status        = "completed"
	job.CompletedAt   = &now
	job.TotalReturn   = res.TotalReturn
	job.AnnualReturn  = res.AnnualReturn
	job.MaxDrawdown   = res.MaxDrawdown
	job.WinRate       = res.WinRate
	job.SharpeRatio   = res.SharpeRatio
	job.TotalTrades   = res.TotalTrades
	job.Progress      = 100
	h.db.Save(&job)

	if len(res.Trades) > 0 {
		h.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(res.Trades, 500)
	}
	if len(res.EquityCurve) > 0 {
		h.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(res.EquityCurve, 1000)
	}

	c.JSON(http.StatusOK, gin.H{
		"job":    job,
		"trades": res.Trades,
		"equity": res.EquityCurve,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/backtest/jobs/:id
// ────────────────────────────────────────────────────────────────────────────
func (h *BacktestHandler) GetJob(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var job models.BacktestJob
	if err := h.db.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	var trades []models.BacktestTrade
	h.db.Where("job_id = ?", id).Order("entry_date ASC").Find(&trades)
	var equity []models.BacktestEquityPoint
	h.db.Where("job_id = ?", id).Order("date ASC").Find(&equity)
	c.JSON(http.StatusOK, gin.H{
		"job":    job,
		"trades": trades,
		"equity": equity,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// DELETE /api/backtest/jobs/:id
// ────────────────────────────────────────────────────────────────────────────
func (h *BacktestHandler) DeleteJob(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	h.db.Where("job_id = ?", id).Delete(&models.BacktestTrade{})
	h.db.Where("job_id = ?", id).Delete(&models.BacktestEquityPoint{})
	h.db.Delete(&models.BacktestJob{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/backtest/defaults
// 回傳預設策略參數
// ────────────────────────────────────────────────────────────────────────────
func (h *BacktestHandler) Defaults(c *gin.Context) {
	c.JSON(http.StatusOK, backtest.DefaultParams())
}
