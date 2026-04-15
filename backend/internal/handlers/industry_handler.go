package handlers

import (
	"net/http"
	"strconv"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type IndustryHandler struct {
	db *gorm.DB
}

func NewIndustryHandler(db *gorm.DB) *IndustryHandler {
	return &IndustryHandler{db: db}
}

// ── 回傳結構 ──────────────────────────────────────────────────────────────────

type IndustryFlowDay struct {
	Date       string `json:"date"`
	ForeignNet int64  `json:"foreign_net"` // 張
	TrustNet   int64  `json:"trust_net"`
	DealerNet  int64  `json:"dealer_net"`
	TotalNet   int64  `json:"total_net"`
}

type IndustryFlowRow struct {
	Industry  string            `json:"industry"`
	Days      []IndustryFlowDay `json:"days"`
	LatestNet int64             `json:"latest_net"` // 最近一日總淨額（排序用）
}

type IndustryStockRow struct {
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Date       string `json:"date"`
	ForeignNet int64  `json:"foreign_net"`
	TrustNet   int64  `json:"trust_net"`
	DealerNet  int64  `json:"dealer_net"`
	TotalNet   int64  `json:"total_net"`
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/industry/flow?days=20
// 回傳過去 N 個交易日、各產業每日的三大法人淨買超張數
// ────────────────────────────────────────────────────────────────────────────
func (h *IndustryHandler) Flow(c *gin.Context) {
	days := 20
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 60 {
			days = n
		}
	}

	// 計算起始日期（往前找 days 個自然日；假日較少，多抓一些再截斷）
	since := time.Now().AddDate(0, 0, -(days * 2))

	type rawRow struct {
		Industry   string    `gorm:"column:industry"`
		Date       time.Time `gorm:"column:date"`
		ForeignNet int64     `gorm:"column:foreign_net"`
		TrustNet   int64     `gorm:"column:trust_net"`
		DealerNet  int64     `gorm:"column:dealer_net"`
		TotalNet   int64     `gorm:"column:total_net"`
	}

	var rows []rawRow
	h.db.Raw(`
		SELECT
		  s.industry,
		  it.date,
		  SUM(it.foreign_net) / 1000                     AS foreign_net,
		  SUM(it.trust_net)   / 1000                     AS trust_net,
		  SUM(it.dealer_net)  / 1000                     AS dealer_net,
		  (SUM(it.foreign_net) + SUM(it.trust_net) + SUM(it.dealer_net)) / 1000 AS total_net
		FROM institutional_tradings it
		JOIN stocks s ON s.symbol = it.symbol
		WHERE it.date >= ?
		  AND s.industry IS NOT NULL
		  AND s.industry <> ''
		GROUP BY s.industry, it.date
		ORDER BY it.date ASC, s.industry ASC
	`, since).Scan(&rows)

	// 收集所有日期（依序去重）
	dateSet := map[string]struct{}{}
	var allDates []string
	for _, r := range rows {
		d := r.Date.Format("2006-01-02")
		if _, ok := dateSet[d]; !ok {
			dateSet[d] = struct{}{}
			allDates = append(allDates, d)
		}
	}
	// 只取最近 days 個交易日
	if len(allDates) > days {
		allDates = allDates[len(allDates)-days:]
	}
	dateIdx := map[string]bool{}
	for _, d := range allDates {
		dateIdx[d] = true
	}

	// 依產業聚合
	type industryMap = map[string]map[string]*IndustryFlowDay
	byIndustry := industryMap{}
	for _, r := range rows {
		d := r.Date.Format("2006-01-02")
		if !dateIdx[d] {
			continue
		}
		if byIndustry[r.Industry] == nil {
			byIndustry[r.Industry] = map[string]*IndustryFlowDay{}
		}
		byIndustry[r.Industry][d] = &IndustryFlowDay{
			Date:       d,
			ForeignNet: r.ForeignNet,
			TrustNet:   r.TrustNet,
			DealerNet:  r.DealerNet,
			TotalNet:   r.TotalNet,
		}
	}

	result := make([]IndustryFlowRow, 0, len(byIndustry))
	for industry, dayMap := range byIndustry {
		var daySlice []IndustryFlowDay
		var latestNet int64
		for _, d := range allDates {
			if v, ok := dayMap[d]; ok {
				daySlice = append(daySlice, *v)
				latestNet = v.TotalNet // 最後一筆即最新
			} else {
				daySlice = append(daySlice, IndustryFlowDay{Date: d})
			}
		}
		result = append(result, IndustryFlowRow{
			Industry:  industry,
			Days:      daySlice,
			LatestNet: latestNet,
		})
	}

	// 依最近一日總淨額排序（大到小）
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].LatestNet > result[i].LatestNet {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"dates": allDates,
		"data":  result,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/industry/flow/:industry?days=5
// 回傳特定產業下個別股票的最近 N 日三大法人資料
// ────────────────────────────────────────────────────────────────────────────
func (h *IndustryHandler) FlowByIndustry(c *gin.Context) {
	industry := c.Param("industry")
	days := 5
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 30 {
			days = n
		}
	}

	since := time.Now().AddDate(0, 0, -(days * 2))

	type rawRow struct {
		Symbol     string    `gorm:"column:symbol"`
		Name       string    `gorm:"column:name"`
		Date       time.Time `gorm:"column:date"`
		ForeignNet int64     `gorm:"column:foreign_net"`
		TrustNet   int64     `gorm:"column:trust_net"`
		DealerNet  int64     `gorm:"column:dealer_net"`
		TotalNet   int64     `gorm:"column:total_net"`
	}
	var rows []rawRow
	h.db.Raw(`
		SELECT
		  it.symbol, s.name, it.date,
		  it.foreign_net / 1000 AS foreign_net,
		  it.trust_net   / 1000 AS trust_net,
		  it.dealer_net  / 1000 AS dealer_net,
		  (it.foreign_net + it.trust_net + it.dealer_net) / 1000 AS total_net
		FROM institutional_tradings it
		JOIN stocks s ON s.symbol = it.symbol
		WHERE s.industry = ?
		  AND it.date >= ?
		ORDER BY it.date DESC, total_net DESC
	`, industry, since).Scan(&rows)

	// 只取最近 days 個日期
	dateSet := map[string]struct{}{}
	var allDates []string
	for _, r := range rows {
		d := r.Date.Format("2006-01-02")
		if _, ok := dateSet[d]; !ok {
			if len(dateSet) >= days {
				break
			}
			dateSet[d] = struct{}{}
			allDates = append(allDates, d)
		}
	}
	dateIdx := map[string]bool{}
	for _, d := range allDates {
		dateIdx[d] = true
	}

	var result []IndustryStockRow
	for _, r := range rows {
		d := r.Date.Format("2006-01-02")
		if !dateIdx[d] {
			continue
		}
		result = append(result, IndustryStockRow{
			Symbol:     r.Symbol,
			Name:       r.Name,
			Date:       d,
			ForeignNet: r.ForeignNet,
			TrustNet:   r.TrustNet,
			DealerNet:  r.DealerNet,
			TotalNet:   r.TotalNet,
		})
	}

	// 取得股票列表（不重複）
	stocksMap := map[string]models.Stock{}
	for _, r := range result {
		stocksMap[r.Symbol] = models.Stock{Symbol: r.Symbol, Name: r.Name}
	}

	c.JSON(http.StatusOK, gin.H{
		"industry": industry,
		"dates":    allDates,
		"data":     result,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/industry/list
// 回傳所有有法人資料的產業清單
// ────────────────────────────────────────────────────────────────────────────
func (h *IndustryHandler) List(c *gin.Context) {
	var industries []string
	h.db.Model(&models.Stock{}).
		Where("industry IS NOT NULL AND industry <> ''").
		Distinct("industry").
		Pluck("industry", &industries)
	c.JSON(http.StatusOK, industries)
}
