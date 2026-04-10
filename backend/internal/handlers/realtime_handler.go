package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RealtimeHandler struct {
	db     *gorm.DB
	client *http.Client
}

func NewRealtimeHandler(db *gorm.DB) *RealtimeHandler {
	return &RealtimeHandler{
		db: db,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RealtimeQuote 即時報價
type RealtimeQuote struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`        // 成交價
	Open        float64 `json:"open"`         // 開盤
	High        float64 `json:"high"`         // 盤中最高
	Low         float64 `json:"low"`          // 盤中最低
	PrevClose   float64 `json:"prev_close"`   // 昨收
	Change      float64 `json:"change"`       // 漲跌
	ChangePct   float64 `json:"change_pct"`   // 漲跌幅 %
	Volume      int64   `json:"volume"`       // 成交量（張）
	TradeTime   string  `json:"trade_time"`   // 最後成交時間 HH:MM:SS
	TradeDate   string  `json:"trade_date"`   // 日期 YYYYMMDD
	IsTrading   bool    `json:"is_trading"`   // 是否有即時數值
}

// twseMsgItem TWSE MIS API 回傳的原始欄位
type twseMsgItem struct {
	C string `json:"c"` // code
	N string `json:"n"` // name
	Z string `json:"z"` // latest price
	O string `json:"o"` // open
	H string `json:"h"` // high
	L string `json:"l"` // low
	Y string `json:"y"` // yesterday close
	V string `json:"v"` // volume (張)
	T string `json:"t"` // time HH:MM:SS
	D string `json:"d"` // date YYYYMMDD
}

type twseResp struct {
	MsgArray []twseMsgItem `json:"msgArray"`
}

func parseF(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseI(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	// 去掉千分位逗號
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// Quote  GET /api/realtime/:symbol
func (h *RealtimeHandler) Quote(c *gin.Context) {
	symbol := c.Param("symbol")

	// 查市場別
	var stock models.Stock
	if err := h.db.Select("symbol, name, market").
		Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "symbol not found"})
		return
	}

	// 決定 exchange prefix
	ex := "tse"
	if stock.Market == "TPEX" {
		ex = "otc"
	}

	url := fmt.Sprintf(
		"https://mis.twse.com.tw/stock/api/getStockInfo.asp?ex_ch=%s_%s.tw&json=1&delay=0",
		ex, symbol,
	)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request build failed"})
		return
	}
	req.Header.Set("Referer", "https://mis.twse.com.tw/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; stock-monitor/1.0)")

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "read upstream body failed"})
		return
	}

	var raw twseResp
	if err := json.Unmarshal(body, &raw); err != nil || len(raw.MsgArray) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream response"})
		return
	}

	item := raw.MsgArray[0]
	price := parseF(item.Z)
	prevClose := parseF(item.Y)
	open := parseF(item.O)
	high := parseF(item.H)
	low := parseF(item.L)

	isTrading := price > 0

	change := 0.0
	changePct := 0.0
	if isTrading && prevClose > 0 {
		change = price - prevClose
		changePct = change / prevClose * 100
	}

	quote := RealtimeQuote{
		Symbol:    symbol,
		Name:      stock.Name,
		Price:     price,
		Open:      open,
		High:      high,
		Low:       low,
		PrevClose: prevClose,
		Change:    change,
		ChangePct: changePct,
		Volume:    parseI(item.V),
		TradeTime: item.T,
		TradeDate: item.D,
		IsTrading: isTrading,
	}

	// Cache-Control: 避免瀏覽器快取即時報價
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, quote)
}
