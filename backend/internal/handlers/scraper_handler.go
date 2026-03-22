package handlers

import (
	"encoding/json"
	"fmt"

	"stock-backend/internal/models"
	"stock-backend/internal/scraper"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ScraperHandler struct {
	db *gorm.DB
}

func NewScraperHandler(db *gorm.DB) *ScraperHandler {
	return &ScraperHandler{db: db}
}

type sseEvent struct {
	Stage    string `json:"stage"`
	Message  string `json:"message,omitempty"`
	Progress int    `json:"progress"`
	URL      string `json:"url,omitempty"`
	Total    int    `json:"total,omitempty"`
	Synced   int    `json:"synced,omitempty"`
	Error    string `json:"error,omitempty"`
}

func writeSSE(c *gin.Context, event sseEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
}

// SyncStocksSSE godoc
// GET /api/scraper/stocks
func (h *ScraperHandler) SyncStocksSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// --- 上市（TWSE）---
	writeSSE(c, sseEvent{
		Stage:    "fetching_listed",
		Message:  "正在向 TWSE OpenAPI 抓取上市股票清單...",
		Progress: 10,
		URL:      scraper.TWSEListedURL,
	})

	listed, err := scraper.FetchListedStocks()
	if err != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: fmt.Sprintf("上市資料抓取失敗：%s", err.Error()), Progress: 10})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "fetched_listed",
		Message:  fmt.Sprintf("上市：已取得 %d 支一般台股", len(listed)),
		Progress: 40,
		Total:    len(listed),
		URL:      scraper.TWSEListedURL,
	})

	// --- 上櫃（TPEX）---
	writeSSE(c, sseEvent{
		Stage:    "fetching_otc",
		Message:  "正在向 TPEX OpenAPI 抓取上櫃股票清單...",
		Progress: 50,
		URL:      scraper.TPEXOtcURL,
	})

	otc, err := scraper.FetchOtcStocks()
	if err != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: fmt.Sprintf("上櫃資料抓取失敗：%s", err.Error()), Progress: 50})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "fetched_otc",
		Message:  fmt.Sprintf("上櫃：已取得 %d 支一般台股", len(otc)),
		Progress: 75,
		Total:    len(otc),
		URL:      scraper.TPEXOtcURL,
	})

	// --- 合併寫入 DB ---
	all := append(listed, otc...)
	stocks := make([]models.Stock, 0, len(all))
	for _, s := range all {
		stocks = append(stocks, models.Stock{
			Symbol: s.Symbol,
			Name:   s.Name,
		})
	}

	writeSSE(c, sseEvent{
		Stage:   "saving",
		Message: fmt.Sprintf("合計 %d 支（上市 %d + 上櫃 %d），寫入資料庫中...", len(stocks), len(listed), len(otc)),
		Progress: 85,
		Synced:  len(stocks),
	})

	result := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "updated_at"}),
	}).Create(&stocks)

	if result.Error != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: result.Error.Error(), Progress: 85})
		return
	}

	writeSSE(c, sseEvent{
		Stage:   "done",
		Message: fmt.Sprintf("同步完成！上市 %d 支 + 上櫃 %d 支，共 %d 支股票", len(listed), len(otc), len(stocks)),
		Progress: 100,
		Synced:  len(stocks),
	})
}
