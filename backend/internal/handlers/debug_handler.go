package handlers

import (
	"net/http"
	"time"

	"stock-backend/internal/models"
	"stock-backend/internal/scraper"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DebugHandler struct {
	db *gorm.DB
}

func NewDebugHandler(db *gorm.DB) *DebugHandler {
	return &DebugHandler{db: db}
}

// RawMonth GET /api/debug/raw-month?symbol=2330&yyyymm=202603
// 回傳單支股票指定月份的原始 API 回應，以及每一列的解析 / 過濾細節
func (h *DebugHandler) RawMonth(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 symbol 參數"})
		return
	}

	yyyymm := c.Query("yyyymm")
	if yyyymm == "" {
		yyyymm = time.Now().In(time.FixedZone("CST", 8*3600)).Format("200601")
	}

	var stock models.Stock
	if err := h.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "找不到此股票，請先執行股票清單同步"})
		return
	}

	result, err := scraper.FetchDebugRawMonth(stock.Symbol, stock.Market, yyyymm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// BrokerFetch GET /api/debug/broker-fetch?symbol=2330
// 依序嘗試所有券商 base URL，回傳第一個成功的原始解析結果（含診斷資訊）
func (h *DebugHandler) BrokerFetch(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 symbol 參數"})
		return
	}

	type response struct {
		Symbol      string   `json:"symbol"`
		Source      string   `json:"source"`
		URL         string   `json:"url"`
		RecordCount int      `json:"record_count"`
		FirstRecord any      `json:"first_record,omitempty"`
		LastRecord  any      `json:"last_record,omitempty"`
		Tried       []string `json:"tried,omitempty"`
		BrokerURLs  []string `json:"broker_urls"`
		Error       string   `json:"error,omitempty"`
	}

	result, err := scraper.FetchBrokerStockHistory(symbol)

	resp := response{
		Symbol:     symbol,
		BrokerURLs: scraper.DefaultBrokerBaseURLs,
		Tried:      result.Tried,
	}

	if err != nil {
		resp.Error = err.Error()
		c.JSON(http.StatusOK, resp) // 回 200 方便前端顯示診斷
		return
	}

	resp.Source = result.Source
	resp.URL = result.URL
	resp.RecordCount = len(result.Records)
	if len(result.Records) > 0 {
		resp.FirstRecord = result.Records[0]
		resp.LastRecord = result.Records[len(result.Records)-1]
	}

	c.JSON(http.StatusOK, resp)
}

