package handlers

import (
	"encoding/json"
	"fmt"
	"time"

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
	for i, s := range all {
		market := "TPEX"
		if i < len(listed) {
			market = "TWSE"
		}
		stocks = append(stocks, models.Stock{
			Symbol:   s.Symbol,
			Name:     s.Name,
			Industry: s.Industry,
			Market:   market,
		})
	}

	writeSSE(c, sseEvent{
		Stage:    "saving",
		Message:  fmt.Sprintf("合計 %d 支（上市 %d + 上櫃 %d），寫入資料庫中...", len(stocks), len(listed), len(otc)),
		Progress: 85,
		Synced:   len(stocks),
	})

	result := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "industry", "market", "updated_at"}),
	}).Create(&stocks)

	if result.Error != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: result.Error.Error(), Progress: 85})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "done",
		Message:  fmt.Sprintf("同步完成！上市 %d 支 + 上櫃 %d 支，共 %d 支股票", len(listed), len(otc), len(stocks)),
		Progress: 100,
		Synced:   len(stocks),
	})
}

// SyncPricesSSE godoc
// GET /api/scraper/prices?date=2025-03-21
// 不傳 date 則使用最近的交易日（今天或上一個工作日）
func (h *ScraperHandler) SyncPricesSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 決定資料日期
	tradingDate := latestTradingDay()
	if d := c.Query("date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			tradingDate = t
		}
	}
	dateStr := tradingDate.Format("2006-01-02")

	// --- TWSE ---
	writeSSE(c, sseEvent{
		Stage:    "fetching_twse",
		Message:  fmt.Sprintf("正在抓取上市日K（%s）...", dateStr),
		Progress: 10,
		URL:      scraper.TWSEDayAllURL,
	})

	listedPrices, err := scraper.FetchTWSEDayAll(tradingDate)
	if err != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: fmt.Sprintf("上市日K 抓取失敗：%s", err.Error()), Progress: 10})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "fetched_twse",
		Message:  fmt.Sprintf("上市取得 %d 筆", len(listedPrices)),
		Progress: 40,
		Total:    len(listedPrices),
		URL:      scraper.TWSEDayAllURL,
	})

	// --- TPEX ---
	writeSSE(c, sseEvent{
		Stage:    "fetching_tpex",
		Message:  fmt.Sprintf("正在抓取上櫃日K（%s）...", dateStr),
		Progress: 50,
		URL:      scraper.TPEXOtcURL,
	})

	otcPrices, err := scraper.FetchTPEXDayAll(tradingDate)
	if err != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: fmt.Sprintf("上櫃日K 抓取失敗：%s", err.Error()), Progress: 50})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "fetched_tpex",
		Message:  fmt.Sprintf("上櫃取得 %d 筆", len(otcPrices)),
		Progress: 70,
		Total:    len(otcPrices),
		URL:      scraper.TPEXOtcURL,
	})

	// --- 寫入 DB（UPSERT by symbol+date）---
	all := append(listedPrices, otcPrices...)
	writeSSE(c, sseEvent{
		Stage:    "saving",
		Message:  fmt.Sprintf("合計 %d 筆，寫入資料庫...", len(all)),
		Progress: 80,
		Synced:   len(all),
	})

	if len(all) > 0 {
		result := h.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "tx_value", "tx_count"}),
		}).CreateInBatches(&all, 500)

		if result.Error != nil {
			writeSSE(c, sseEvent{Stage: "error", Error: result.Error.Error(), Progress: 80})
			return
		}
	}

	writeSSE(c, sseEvent{
		Stage:    "done",
		Message:  fmt.Sprintf("日K 同步完成！%s 上市 %d 筆 + 上櫃 %d 筆", dateStr, len(listedPrices), len(otcPrices)),
		Progress: 100,
		Synced:   len(all),
	})
}

// latestTradingDay 回傳最近的交易日（跳過週末）
func latestTradingDay() time.Time {
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	// 若目前時間在 15:00 前，資料可能尚未更新，用前一個交易日
	if now.Hour() < 15 {
		now = now.AddDate(0, 0, -1)
	}
	// 跳過週末
	for now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		now = now.AddDate(0, 0, -1)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// RefreshStockSSE godoc
// GET /api/scraper/prices/stock/:symbol
// 抓取單支股票近 3 個月的日K 資料並更新資料庫，以 SSE 回傳進度
func (h *ScraperHandler) RefreshStockSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	symbol := c.Param("symbol")
	if symbol == "" {
		writeSSE(c, sseEvent{Stage: "error", Error: "缺少股票代號", Progress: 0})
		return
	}

	// 查詢市場別（TWSE / TPEX）
	var stock models.Stock
	if err := h.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: fmt.Sprintf("找不到股票 %s", symbol), Progress: 0})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "start",
		Message:  fmt.Sprintf("開始更新 %s（%s）近 3 個月資料...", symbol, stock.Market),
		Progress: 5,
	})

	// 計算近 3 個月的年月列表
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	months := make([]string, 0, 3)
	for i := 2; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		months = append(months, fmt.Sprintf("%d%02d", t.Year(), t.Month()))
	}

	var all []models.DailyPrice
	for idx, ym := range months {
		progress := 10 + (idx+1)*25
		writeSSE(c, sseEvent{
			Stage:    "fetching",
			Message:  fmt.Sprintf("抓取 %s/%s 資料中...", symbol, ym),
			Progress: progress,
		})

		var records []models.DailyPrice
		var fetchErr error

		if stock.Market == "TWSE" {
			records, fetchErr = scraper.FetchTWSEStockHistory(symbol, ym)
		} else {
			records, fetchErr = scraper.FetchTPEXStockHistory(symbol, ym)
		}

		if fetchErr != nil {
			// 單月失敗不中止，繼續抓其他月份
			writeSSE(c, sseEvent{
				Stage:    "warning",
				Message:  fmt.Sprintf("%s/%s 抓取失敗：%s，跳過", symbol, ym, fetchErr.Error()),
				Progress: progress,
			})
			continue
		}
		all = append(all, records...)
		writeSSE(c, sseEvent{
			Stage:    "fetched",
			Message:  fmt.Sprintf("%s/%s 取得 %d 筆", symbol, ym, len(records)),
			Progress: progress + 5,
			Total:    len(records),
		})
	}

	if len(all) == 0 {
		writeSSE(c, sseEvent{Stage: "error", Error: "未取得任何資料，可能為假日或股票代號錯誤", Progress: 90})
		return
	}

	writeSSE(c, sseEvent{
		Stage:    "saving",
		Message:  fmt.Sprintf("共 %d 筆，寫入資料庫...", len(all)),
		Progress: 90,
		Synced:   len(all),
	})

	result := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "tx_value", "tx_count"}),
	}).CreateInBatches(&all, 200)

	if result.Error != nil {
		writeSSE(c, sseEvent{Stage: "error", Error: result.Error.Error(), Progress: 90})
		return
	}

	writeSSE(c, sseEvent{
		Stage:   "done",
		Message: fmt.Sprintf("%s 更新完成！共 %d 筆日K 資料", symbol, len(all)),
		Progress: 100,
		Synced:  len(all),
	})
}
