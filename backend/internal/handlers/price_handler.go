package handlers

import (
	"net/http"
	"strconv"
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
