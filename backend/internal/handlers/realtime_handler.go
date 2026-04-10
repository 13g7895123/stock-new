package handlers

import (
"encoding/json"
"fmt"
"io"
"net/http"
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
Timeout: 8 * time.Second,
},
}
}

// RealtimeQuote 即時報價
type RealtimeQuote struct {
Symbol    string  `json:"symbol"`
Name      string  `json:"name"`
Price     float64 `json:"price"`      // 成交價
Open      float64 `json:"open"`       // 開盤
High      float64 `json:"high"`       // 盤中最高
Low       float64 `json:"low"`        // 盤中最低
PrevClose float64 `json:"prev_close"` // 昨收
Change    float64 `json:"change"`     // 漲跌
ChangePct float64 `json:"change_pct"` // 漲跌幅 %
Volume    int64   `json:"volume"`     // 成交量（張）
TradeTime string  `json:"trade_time"` // 最後成交時間 HH:MM:SS
TradeDate string  `json:"trade_date"` // 日期 YYYYMMDD
IsTrading bool    `json:"is_trading"` // 是否有即時數值
}

// yahooChartMeta Yahoo Finance v8 chart API meta field
type yahooChartMeta struct {
ShortName            string  `json:"shortName"`
LongName             string  `json:"longName"`
RegularMarketPrice   float64 `json:"regularMarketPrice"`
RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
RegularMarketVolume  int64   `json:"regularMarketVolume"`
RegularMarketTime    int64   `json:"regularMarketTime"`
ChartPreviousClose   float64 `json:"chartPreviousClose"`
CurrentTradingPeriod struct {
Regular struct {
Start int64 `json:"start"`
End   int64 `json:"end"`
} `json:"regular"`
} `json:"currentTradingPeriod"`
}

type yahooChartQuote struct {
Open   []float64 `json:"open"`
High   []float64 `json:"high"`
Low    []float64 `json:"low"`
Close  []float64 `json:"close"`
Volume []int64   `json:"volume"`
}

type yahooChartResult struct {
Meta       yahooChartMeta `json:"meta"`
Timestamps []int64        `json:"timestamp"`
Indicators struct {
Quote []yahooChartQuote `json:"quote"`
} `json:"indicators"`
}

type yahooChartResp struct {
Chart struct {
Result []yahooChartResult `json:"result"`
Error  interface{}        `json:"error"`
} `json:"chart"`
}

// Quote  GET /api/realtime/:symbol
// 使用 Yahoo Finance v8 chart API：無需認證，可從伺服器端呼叫
func (h *RealtimeHandler) Quote(c *gin.Context) {
symbol := c.Param("symbol")

// 查市場別
var stock models.Stock
if err := h.db.Select("symbol, name, market").
Where("symbol = ?", symbol).First(&stock).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "symbol not found"})
return
}

// Yahoo Finance 台股 suffix：上市 .TW，上櫃 .TWO
suffix := ".TW"
if stock.Market == "TPEX" {
suffix = ".TWO"
}
yahooSym := symbol + suffix

url := fmt.Sprintf(
"https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=1d",
yahooSym,
)

req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "request build failed"})
return
}
req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
req.Header.Set("Accept", "application/json")

resp, err := h.client.Do(req)
if err != nil {
c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
return
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
if err != nil {
c.JSON(http.StatusBadGateway, gin.H{"error": "read upstream body failed"})
return
}

var raw yahooChartResp
if err := json.Unmarshal(body, &raw); err != nil {
c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream response"})
return
}
if len(raw.Chart.Result) == 0 {
c.JSON(http.StatusBadGateway, gin.H{"error": "no data for symbol"})
return
}

meta := raw.Chart.Result[0].Meta
quotes := raw.Chart.Result[0].Indicators.Quote

// 取今日開盤價
openPrice := 0.0
if len(quotes) > 0 && len(quotes[0].Open) > 0 {
openPrice = quotes[0].Open[0]
}

// 判斷是否盤中：現在時間介於交易時段 start ~ end
now := time.Now().Unix()
regStart := meta.CurrentTradingPeriod.Regular.Start
regEnd := meta.CurrentTradingPeriod.Regular.End
isTrading := now >= regStart && now <= regEnd && meta.RegularMarketPrice > 0

tradeTime := ""
tradeDate := ""
if meta.RegularMarketTime > 0 {
t := time.Unix(meta.RegularMarketTime, 0).In(time.FixedZone("CST", 8*3600))
tradeTime = t.Format("15:04:05")
tradeDate = t.Format("2006-01-02")
}

prevClose := meta.ChartPreviousClose
price := meta.RegularMarketPrice
change := 0.0
changePct := 0.0
if prevClose > 0 {
change = price - prevClose
changePct = change / prevClose * 100
}

// Yahoo volume 單位是「股」，台股 1 張 = 1000 股
volumeLot := meta.RegularMarketVolume / 1000

quote := RealtimeQuote{
Symbol:    symbol,
Name:      stock.Name,
Price:     price,
Open:      openPrice,
High:      meta.RegularMarketDayHigh,
Low:       meta.RegularMarketDayLow,
PrevClose: prevClose,
Change:    change,
ChangePct: changePct,
Volume:    volumeLot,
TradeTime: tradeTime,
TradeDate: tradeDate,
IsTrading: isTrading,
}

c.Header("Cache-Control", "no-store")
c.JSON(http.StatusOK, quote)
}
