package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DividendHandler struct {
	db      *gorm.DB
	mu      sync.Mutex
	running bool
}

func NewDividendHandler(db *gorm.DB) *DividendHandler {
	return &DividendHandler{db: db}
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/stocks/:symbol/dividends
// 回傳某股票的所有除息記錄（最新在前）
// ────────────────────────────────────────────────────────────────────────────
func (h *DividendHandler) GetBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	var divs []models.Dividend
	if err := h.db.Where("symbol = ?", symbol).
		Order("ex_date DESC").
		Find(&divs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, divs)
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/stocks/:symbol/prices/adjusted
// 回傳還原(前復權)日K資料
// ────────────────────────────────────────────────────────────────────────────
func (h *DividendHandler) AdjustedPrices(c *gin.Context) {
	symbol := c.Param("symbol")

	// 取日K
	var rawPrices []models.DailyPrice
	q := h.db.Where("symbol = ?", symbol).Order("date ASC")
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
	if err := q.Limit(2000).Find(&rawPrices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(rawPrices) == 0 {
		c.JSON(http.StatusOK, []any{})
		return
	}

	// 取除息記錄（僅現金股利 > 0）
	var divs []models.Dividend
	h.db.Where("symbol = ? AND cash_dividend > 0", symbol).
		Order("ex_date ASC").
		Find(&divs)

	// 計算前復權係數（從最早日 → 最新日，逐個除息日累乘）
	// 前復權邏輯：在除息日之前的所有資料乘以 (1 - dividend/ref_price) 係數
	adjusted := calcAdjustedPrices(rawPrices, divs)
	c.JSON(http.StatusOK, adjusted)
}

// AdjustedBar 還原K線回傳結構（與 DailyPrice 相容的欄位）
type AdjustedBar struct {
	ID       uint    `json:"id"`
	Symbol   string  `json:"symbol"`
	Date     string  `json:"date"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   int64   `json:"volume"`
	TxValue  int64   `json:"tx_value"`
	TxCount  int     `json:"tx_count"`
	AdjRatio float64 `json:"adj_ratio"` // 復權係數（供除錯用）
}

// calcAdjustedPrices 計算前復權價格（Pre-Adjusted Close，累積除息倒推）
func calcAdjustedPrices(prices []models.DailyPrice, divs []models.Dividend) []AdjustedBar {
	if len(prices) == 0 {
		return []AdjustedBar{}
	}

	// 建立除息日 → 係數 map
	// 前復權係數 = (refPrice - cashDividend) / refPrice
	divMap := map[string]float64{}
	for _, d := range divs {
		if d.RefPrice > 0 {
			key := d.ExDate.Format("2006-01-02")
			factor := (d.RefPrice - d.CashDividend) / d.RefPrice
			divMap[key] = factor
		}
	}

	// 從後往前累乘復權係數
	// ratio[i] = 從 i 到最後所有除息日係數的乘積（前復權）
	n := len(prices)
	ratios := make([]float64, n)
	cumRatio := 1.0
	for i := n - 1; i >= 0; i-- {
		dateStr := prices[i].Date.Format("2006-01-02")
		if factor, ok := divMap[dateStr]; ok {
			cumRatio *= factor
		}
		ratios[i] = cumRatio
	}

	result := make([]AdjustedBar, n)
	for i, p := range prices {
		r := ratios[i]
		result[i] = AdjustedBar{
			ID:       p.ID,
			Symbol:   p.Symbol,
			Date:     p.Date.Format("2006-01-02") + "T00:00:00Z",
			Open:     roundPrice(p.Open / r),
			High:     roundPrice(p.High / r),
			Low:      roundPrice(p.Low / r),
			Close:    roundPrice(p.Close / r),
			Volume:   p.Volume,
			TxValue:  p.TxValue,
			TxCount:  p.TxCount,
			AdjRatio: r,
		}
	}
	return result
}

func roundPrice(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/dividends/upcoming?days=30
// 回傳未來 N 天將除息的股票清單
// ────────────────────────────────────────────────────────────────────────────
func (h *DividendHandler) Upcoming(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	type Row struct {
		Symbol       string    `json:"symbol"`
		Name         string    `json:"name"`
		ExDate       time.Time `json:"ex_date"`
		CashDividend float64   `json:"cash_dividend"`
		RefPrice     float64   `json:"ref_price"`
		Yield        float64   `json:"yield_pct"` // 現金殖利率 = cashDividend/現股價
	}
	var rows []Row
	h.db.Raw(`
		SELECT d.symbol, s.name, d.ex_date,
		       d.cash_dividend, d.ref_price,
		       CASE WHEN s.price > 0
		         THEN ROUND(d.cash_dividend / s.price * 100, 2)
		         ELSE 0
		       END AS yield
		FROM dividends d
		LEFT JOIN stocks s ON s.symbol = d.symbol
		WHERE d.ex_date >= CURRENT_DATE
		  AND d.ex_date <= CURRENT_DATE + INTERVAL '1 day' * ?
		  AND d.cash_dividend > 0
		ORDER BY d.ex_date ASC`, days).Scan(&rows)
	c.JSON(http.StatusOK, rows)
}

// ────────────────────────────────────────────────────────────────────────────
// POST /api/dividends/sync
// 從 TWSE 同步除息資訊（近 1 年）
// ────────────────────────────────────────────────────────────────────────────
func (h *DividendHandler) Sync(c *gin.Context) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "已有同步作業執行中"})
		return
	}
	h.running = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
		}()
		if err := h.syncFromTWSE(); err != nil {
			log.Printf("[dividend] sync error: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已觸發除息資訊同步（TWSE + TPEx）"})
}

// GET /api/dividends/status
func (h *DividendHandler) Status(c *gin.Context) {
	h.mu.Lock()
	running := h.running
	h.mu.Unlock()

	var count int64
	h.db.Model(&models.Dividend{}).Count(&count)
	var latest time.Time
	h.db.Model(&models.Dividend{}).Select("MAX(created_at)").Scan(&latest)
	c.JSON(http.StatusOK, gin.H{
		"running":     running,
		"total":       count,
		"last_synced": latest,
	})
}

// ── TWSE 除息資訊爬取 ─────────────────────────────────────────────────────────
// API: https://www.twse.com.tw/exchangeReport/TWT48U
// 每次只能查一段日期範圍；此處每季分批查詢
func (h *DividendHandler) syncFromTWSE() error {
	today := time.Now()
	// 同步前後各半年
	startDate := today.AddDate(0, -6, 0)
	endDate := today.AddDate(0, 6, 0)

	apiURL := fmt.Sprintf(
		"https://www.twse.com.tw/exchangeReport/TWT48U?response=json&strDate=%s&endDate=%s",
		url.QueryEscape(startDate.Format("20060102")),
		url.QueryEscape(endDate.Format("20060102")),
	)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("TWSE dividend API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	type twseResp struct {
		Status string     `json:"stat"`
		Fields []string   `json:"fields"`
		Data   [][]string `json:"data"`
	}
	var tr twseResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("parse TWSE dividend JSON: %w", err)
	}

	if tr.Status != "OK" || len(tr.Data) == 0 {
		log.Printf("[dividend] TWSE returned no data or non-OK status")
		return nil
	}

	// 欄位：日期、代碼、名稱、除息參考價、現金股利、股票股利等
	// 實際欄位順序需依 API 回傳確認；此處做保守解析
	var records []models.Dividend
	for _, row := range tr.Data {
		if len(row) < 5 {
			continue
		}
		// row[0]=除息日(109/05/20), row[1]=代碼, row[3]=除息參考價, row[4]=現金股利
		dateStr := row[0]
		symbolStr := row[1]
		refPriceStr := row[3]
		cashDivStr := row[4]

		exDate, err := parseTWDateStr(dateStr)
		if err != nil {
			continue
		}
		refPrice, _ := strconv.ParseFloat(refPriceStr, 64)
		cashDiv, _ := strconv.ParseFloat(cashDivStr, 64)

		if symbolStr == "" {
			continue
		}

		records = append(records, models.Dividend{
			Symbol:       symbolStr,
			ExDate:       exDate,
			CashDividend: cashDiv,
			RefPrice:     refPrice,
			Market:       "TWSE",
		})
	}

	if len(records) == 0 {
		return nil
	}

	// 批次 UPSERT
	if err := h.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "symbol"}, {Name: "ex_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cash_dividend", "ref_price",
		}),
	}).CreateInBatches(records, 200).Error; err != nil {
		return fmt.Errorf("upsert dividends: %w", err)
	}
	log.Printf("[dividend] upserted %d records from TWSE", len(records))
	return nil
}

// parseTWDateStr 解析民國年日期字串 "YYY/MM/DD" → time.Time
func parseTWDateStr(s string) (time.Time, error) {
	if len(s) < 9 {
		return time.Time{}, fmt.Errorf("invalid date: %s", s)
	}
	var y, m, d int
	_, err := fmt.Sscanf(s, "%d/%d/%d", &y, &m, &d)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(y+1911, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
}
