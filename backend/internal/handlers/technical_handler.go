package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TechnicalHandler struct {
	db *gorm.DB
}

func NewTechnicalHandler(db *gorm.DB) *TechnicalHandler {
	return &TechnicalHandler{db: db}
}

// TechnicalResult 單支股票的技術分析結果
type TechnicalResult struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	LatestClose   float64 `json:"latest_close"`
	LatestDate    string  `json:"latest_date"`
	PrevWeekHigh  float64 `json:"prev_week_high"`
	PrevMonthHigh float64 `json:"prev_month_high"`
	AboveWeekHigh bool    `json:"above_week_high"`
	AboveMonthHigh bool   `json:"above_month_high"`
	NearWeekHigh  bool    `json:"near_week_high"`
	NearMonthHigh bool    `json:"near_month_high"`
}

// Screener  GET /api/technical/screener
// 回傳所有有日K資料的股票，並附帶技術篩選欄位：
//   - above_week_high:  收盤 > 前5根K棒最高收盤
//   - above_month_high: 收盤 > 前20根K棒最高收盤
//   - near_week_high:   收盤 ≤ 前5根高點，但 收盤×1.1 ≥ 前5根高點
//   - near_month_high:  收盤 ≤ 前20根高點，但 收盤×1.1 ≥ 前20根高點
func (h *TechnicalHandler) Screener(c *gin.Context) {
	type row struct {
		Symbol        string  `gorm:"column:symbol"`
		Name          string  `gorm:"column:name"`
		LatestClose   float64 `gorm:"column:latest_close"`
		LatestDate    string  `gorm:"column:latest_date"`
		PrevWeekHigh  float64 `gorm:"column:prev_week_high"`
		PrevMonthHigh float64 `gorm:"column:prev_month_high"`
	}

	const query = `
WITH latest_date AS (
    SELECT symbol, MAX(date) AS max_date
    FROM daily_prices
    GROUP BY symbol
),
latest_price AS (
    SELECT dp.symbol, dp.close AS latest_close, TO_CHAR(dp.date, 'YYYY-MM-DD') AS latest_date
    FROM daily_prices dp
    JOIN latest_date ld ON dp.symbol = ld.symbol AND dp.date = ld.max_date
),
prev_ranked AS (
    SELECT dp.symbol, dp.close,
           ROW_NUMBER() OVER (PARTITION BY dp.symbol ORDER BY dp.date DESC) AS rn
    FROM daily_prices dp
    JOIN latest_date ld ON dp.symbol = ld.symbol AND dp.date < ld.max_date
),
week_high AS (
    SELECT symbol, MAX(close) AS prev_week_high
    FROM prev_ranked
    WHERE rn <= 5
    GROUP BY symbol
),
month_high AS (
    SELECT symbol, MAX(close) AS prev_month_high
    FROM prev_ranked
    WHERE rn <= 20
    GROUP BY symbol
)
SELECT
    s.symbol,
    s.name,
    lp.latest_close,
    lp.latest_date,
    COALESCE(wh.prev_week_high, 0)  AS prev_week_high,
    COALESCE(mh.prev_month_high, 0) AS prev_month_high
FROM stocks s
JOIN latest_price lp ON s.symbol = lp.symbol
LEFT JOIN week_high  wh ON s.symbol = wh.symbol
LEFT JOIN month_high mh ON s.symbol = mh.symbol
ORDER BY s.symbol
`

	var rows []row
	if err := h.db.Raw(query).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]TechnicalResult, 0, len(rows))
	for _, r := range rows {
		aboveWeek := r.PrevWeekHigh > 0 && r.LatestClose > r.PrevWeekHigh
		aboveMonth := r.PrevMonthHigh > 0 && r.LatestClose > r.PrevMonthHigh

		nearWeek := !aboveWeek && r.PrevWeekHigh > 0 &&
			r.LatestClose*1.10 >= r.PrevWeekHigh
		nearMonth := !aboveMonth && r.PrevMonthHigh > 0 &&
			r.LatestClose*1.10 >= r.PrevMonthHigh

		results = append(results, TechnicalResult{
			Symbol:        r.Symbol,
			Name:          r.Name,
			LatestClose:   r.LatestClose,
			LatestDate:    r.LatestDate,
			PrevWeekHigh:  r.PrevWeekHigh,
			PrevMonthHigh: r.PrevMonthHigh,
			AboveWeekHigh:  aboveWeek,
			AboveMonthHigh: aboveMonth,
			NearWeekHigh:  nearWeek,
			NearMonthHigh: nearMonth,
		})
	}

	c.JSON(http.StatusOK, results)
}
