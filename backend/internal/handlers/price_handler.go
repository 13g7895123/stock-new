package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PriceHandler struct {
	db *gorm.DB
}

func NewPriceHandler(db *gorm.DB) *PriceHandler {
	return &PriceHandler{db: db}
}

// AggregatedBar 用於週K / 月K 回傳結構
type AggregatedBar struct {
	PeriodStart string  `json:"period_start"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      int64   `json:"volume"`
	TxValue     int64   `json:"tx_value"`
}

// PreviousTradingDayPrice 用於回傳最近兩個交易日的收盤價與成交量
type PreviousTradingDayPrice struct {
	Date   string  `json:"date"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// StockStatusFlag 用於回傳股票目前有效的處置 / 注意 / 限當沖標記。
type StockStatusFlag struct {
	Type       string `json:"type"`
	Label      string `json:"label"`
	SourceDate string `json:"source_date"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Reason     string `json:"reason,omitempty"`
	Measure    string `json:"measure,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// MarketPreviousTradingDaysStock 用於回傳單檔股票最近兩個交易日價量
type MarketPreviousTradingDaysStock struct {
	Symbol               string                    `json:"symbol"`
	Name                 string                    `json:"name"`
	Market               string                    `json:"market"`
	IsDisposition        bool                      `json:"is_disposition"`
	IsAttention          bool                      `json:"is_attention"`
	IsDayTradeRestricted bool                      `json:"is_day_trade_restricted"`
	Statuses             []StockStatusFlag         `json:"statuses"`
	Data                 []PreviousTradingDayPrice `json:"data"`
}

// MarketPreviousTradingDaysResponse 全市場最近兩個交易日價量回傳結構
type MarketPreviousTradingDaysResponse struct {
	AsOf  string                           `json:"as_of"`
	Count int                              `json:"count"`
	Data  []MarketPreviousTradingDaysStock `json:"data"`
}

// Aggregated  GET /api/stocks/:symbol/prices/aggregated?period=weekly|monthly&from=YYYY-MM-DD&to=YYYY-MM-DD
// 回傳週K（period=weekly）或月K（period=monthly）聚合資料
func (h *PriceHandler) Aggregated(c *gin.Context) {
	symbol := c.Param("symbol")
	period := c.DefaultQuery("period", "weekly")
	if period != "weekly" && period != "monthly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be 'weekly' or 'monthly'"})
		return
	}

	truncUnit := "week"
	if period == "monthly" {
		truncUnit = "month"
	}

	q := h.db.Model(&models.DailyPrice{}).Where("symbol = ?", symbol)
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("date >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("date <= ?", t)
		}
	}
	// 取出原始日K 用於聚合
	var prices []models.DailyPrice
	if err := q.Order("date ASC").Limit(5000).Find(&prices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(prices) == 0 {
		c.JSON(http.StatusOK, []AggregatedBar{})
		return
	}

	// 在 Go 層做聚合（避免 DB SQL 函數差異）
	bars := aggregatePrices(prices, truncUnit)
	c.JSON(http.StatusOK, bars)
}

// aggregatePrices 依 truncUnit ("week"|"month") 聚合日K → 週K/月K
func aggregatePrices(prices []models.DailyPrice, truncUnit string) []AggregatedBar {
	type group struct {
		open    float64
		high    float64
		low     float64
		close   float64
		volume  int64
		txValue int64
		count   int
	}
	// 使用有序 key slice 保持順序
	keyOrder := []string{}
	groups := map[string]*group{}

	for _, p := range prices {
		var key string
		if truncUnit == "week" {
			// 每週以「週一」為基準
			wd := int(p.Date.Weekday())
			if wd == 0 {
				wd = 7
			}
			mon := p.Date.AddDate(0, 0, -(wd - 1))
			key = mon.Format("2006-01-02")
		} else {
			key = p.Date.Format("2006-01")
		}

		if _, exists := groups[key]; !exists {
			groups[key] = &group{
				open: p.Open,
				high: p.High,
				low:  p.Low,
			}
			keyOrder = append(keyOrder, key)
		}
		g := groups[key]
		if p.High > g.high {
			g.high = p.High
		}
		if p.Low < g.low {
			g.low = p.Low
		}
		g.close = p.Close // 最後一根即為本期收盤
		g.volume += p.Volume
		g.txValue += p.TxValue
		g.count++
	}

	bars := make([]AggregatedBar, 0, len(keyOrder))
	for _, k := range keyOrder {
		g := groups[k]
		bars = append(bars, AggregatedBar{
			PeriodStart: k,
			Open:        g.open,
			High:        g.high,
			Low:         g.low,
			Close:       g.close,
			Volume:      g.volume,
			TxValue:     g.txValue,
		})
	}
	return bars
}

// MarketPreviousTradingDays  GET /api/prices/previous-trading-days?as_of=YYYY-MM-DD&market=TWSE|TPEX
// 回傳截至 as_of（預設今天）全市場每檔股票最近兩個有日K資料的交易日收盤價與成交量。
func (h *PriceHandler) MarketPreviousTradingDays(c *gin.Context) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.Local
	}
	asOf := time.Now().In(loc)
	if raw := c.Query("as_of"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
			return
		}
		asOf = parsed
	}
	asOfDate := asOf.Format("2006-01-02")
	market := strings.ToUpper(strings.TrimSpace(c.Query("market")))
	if market != "" && market != "TWSE" && market != "TPEX" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "market must be TWSE or TPEX"})
		return
	}

	type row struct {
		Symbol string    `gorm:"column:symbol"`
		Name   string    `gorm:"column:name"`
		Market string    `gorm:"column:market"`
		Date   time.Time `gorm:"column:date"`
		Close  float64   `gorm:"column:close"`
		Volume int64     `gorm:"column:volume"`
	}

	query := `
SELECT
    s.symbol,
    s.name,
    s.market,
	dp.date,
	dp.close,
	dp.volume
FROM stocks s
JOIN LATERAL (
	SELECT date, close, volume
	FROM daily_prices dp
	WHERE dp.symbol = s.symbol
	  AND dp.date <= ?
	ORDER BY dp.date DESC
	LIMIT 2
) dp ON TRUE
WHERE s.deleted_at IS NULL
`
	args := []any{asOfDate}
	if market != "" {
		query += "  AND s.market = ?\n"
		args = append(args, market)
	} else {
		query += "  AND s.market IN ('TWSE', 'TPEX')\n"
	}
	query += "ORDER BY s.symbol, dp.date DESC"

	var rows []row
	if err := h.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	statusMap, err := h.loadActiveStatusFlags(asOfDate, market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, MarketPreviousTradingDaysResponse{
			AsOf:  asOfDate,
			Count: 0,
			Data:  []MarketPreviousTradingDaysStock{},
		})
		return
	}

	stocks := make([]MarketPreviousTradingDaysStock, 0)
	stockIndex := make(map[string]int)
	for _, r := range rows {
		index, ok := stockIndex[r.Symbol]
		if !ok {
			index = len(stocks)
			stockIndex[r.Symbol] = index
			statuses := statusMap[r.Symbol]
			if statuses == nil {
				statuses = []StockStatusFlag{}
			}
			stocks = append(stocks, MarketPreviousTradingDaysStock{
				Symbol:               r.Symbol,
				Name:                 r.Name,
				Market:               r.Market,
				IsDisposition:        hasStatusType(statuses, models.StockStatusDisposition),
				IsAttention:          hasStatusType(statuses, models.StockStatusAttention),
				IsDayTradeRestricted: hasStatusType(statuses, models.StockStatusDayTradeRestricted),
				Statuses:             statuses,
				Data:                 []PreviousTradingDayPrice{},
			})
		}

		stocks[index].Data = append(stocks[index].Data, PreviousTradingDayPrice{
			Date:   r.Date.Format("2006-01-02"),
			Close:  r.Close,
			Volume: r.Volume,
		})
	}

	c.JSON(http.StatusOK, MarketPreviousTradingDaysResponse{
		AsOf:  asOfDate,
		Count: len(stocks),
		Data:  stocks,
	})
}

func (h *PriceHandler) loadActiveStatusFlags(asOfDate, market string) (map[string][]StockStatusFlag, error) {
	q := h.db.Model(&models.StockStatus{}).
		Where("start_date <= ? AND end_date >= ?", asOfDate, asOfDate).
		Order("symbol ASC, type ASC, start_date ASC")
	if market != "" {
		q = q.Where("market = ?", market)
	} else {
		q = q.Where("market IN ?", []string{"TWSE", "TPEX"})
	}

	var statuses []models.StockStatus
	if err := q.Find(&statuses).Error; err != nil {
		return nil, err
	}

	out := make(map[string][]StockStatusFlag, len(statuses))
	for _, status := range statuses {
		out[status.Symbol] = append(out[status.Symbol], StockStatusFlag{
			Type:       status.Type,
			Label:      statusLabel(status.Type),
			SourceDate: status.SourceDate.Format("2006-01-02"),
			StartDate:  status.StartDate.Format("2006-01-02"),
			EndDate:    status.EndDate.Format("2006-01-02"),
			Reason:     status.Reason,
			Measure:    status.Measure,
			Detail:     status.Detail,
		})
	}
	return out, nil
}

func hasStatusType(statuses []StockStatusFlag, statusType string) bool {
	for _, status := range statuses {
		if status.Type == statusType {
			return true
		}
	}
	return false
}

func statusLabel(statusType string) string {
	switch statusType {
	case models.StockStatusDisposition:
		return "處置股"
	case models.StockStatusAttention:
		return "注意股"
	case models.StockStatusDayTradeRestricted:
		return "限當沖股"
	default:
		return statusType
	}
}

// List  GET /api/stocks/:symbol/prices?from=2024-01-01&to=2024-12-31&limit=500
func (h *PriceHandler) List(c *gin.Context) {
	symbol := c.Param("symbol")

	q := h.db.Model(&models.DailyPrice{}).
		Where("symbol = ?", symbol).
		Order("date ASC")

	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("date >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("date <= ?", t)
		}
	}

	limit := 500
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}

	var prices []models.DailyPrice
	if err := q.Limit(limit).Find(&prices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prices)
}

// Latest  GET /api/stocks/:symbol/prices/latest
func (h *PriceHandler) Latest(c *gin.Context) {
	symbol := c.Param("symbol")
	var price models.DailyPrice
	if err := h.db.Where("symbol = ?", symbol).Order("date DESC").First(&price).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no price data"})
		return
	}
	c.JSON(http.StatusOK, price)
}
