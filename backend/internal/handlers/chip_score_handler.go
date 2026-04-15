package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChipScoreHandler struct {
	db  *gorm.DB
	mu  sync.Mutex
	running bool
}

func NewChipScoreHandler(db *gorm.DB) *ChipScoreHandler {
	return &ChipScoreHandler{db: db}
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/chip-scores?limit=100&sort=score
// 回傳全市場籌碼評分排行
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) List(c *gin.Context) {
	limit := 200
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	sort := c.DefaultQuery("sort", "score") // score | symbol
	orderCol := "total_score DESC"
	if sort == "symbol" {
		orderCol = "symbol ASC"
	}

	// 聯結 stocks 取得公司名稱
	type Row struct {
		Symbol             string    `json:"symbol"`
		Name               string    `json:"name"`
		Industry           string    `json:"industry"`
		CalcDate           time.Time `json:"calc_date"`
		TotalScore         float64   `json:"total_score"`
		InstitutionalScore float64   `json:"institutional_score"`
		MajorScore         float64   `json:"major_score"`
		ChipsPyramidScore  float64   `json:"chips_pyramid_score"`
		WinrateScore       float64   `json:"winrate_score"`
	}
	var rows []Row
	err := h.db.Raw(`
		SELECT cs.symbol, s.name, s.industry,
		       cs.calc_date, cs.total_score,
		       cs.institutional_score, cs.major_score,
		       cs.chips_pyramid_score, cs.winrate_score
		FROM stock_chip_scores cs
		LEFT JOIN stocks s ON s.symbol = cs.symbol
		ORDER BY `+orderCol+`
		LIMIT ?`, limit).Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "count": len(rows)})
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/stocks/:symbol/chip-score
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) GetBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	var score models.StockChipScore
	if err := h.db.Where("symbol = ?", symbol).First(&score).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚無籌碼評分，請先觸發計算"})
		return
	}
	c.JSON(http.StatusOK, score)
}

// ────────────────────────────────────────────────────────────────────────────
// POST /api/chip-scores/calc
// 觸發全市場批次計算
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) Trigger(c *gin.Context) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "已有計算作業執行中"})
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
		if err := h.calcAll(""); err != nil {
			log.Printf("[chip-score] calc all error: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已觸發全市場籌碼評分計算"})
}

// ────────────────────────────────────────────────────────────────────────────
// POST /api/chip-scores/calc/:symbol
// 觸發單股計算
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) TriggerSingle(c *gin.Context) {
	symbol := c.Param("symbol")
	score, err := h.calcOne(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, score)
}

// ────────────────────────────────────────────────────────────────────────────
// GET /api/chip-scores/status
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) Status(c *gin.Context) {
	h.mu.Lock()
	running := h.running
	h.mu.Unlock()

	var latest models.StockChipScore
	h.db.Order("updated_at DESC").First(&latest)
	c.JSON(http.StatusOK, gin.H{
		"running":   running,
		"last_calc": latest.CalcDate,
		"count":     h.db.Model(&models.StockChipScore{}).RowsAffected,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// calcAll 批次計算所有股票（空 symbol = 計算全部）
// ────────────────────────────────────────────────────────────────────────────
func (h *ChipScoreHandler) calcAll(filterSymbol string) error {
	var symbols []string
	q := h.db.Model(&models.Stock{}).Pluck("symbol", &symbols)
	if filterSymbol != "" {
		q = h.db.Model(&models.Stock{}).Where("symbol = ?", filterSymbol).Pluck("symbol", &symbols)
	}
	if q.Error != nil {
		return q.Error
	}

	log.Printf("[chip-score] 開始計算 %d 支股票籌碼評分", len(symbols))
	done := 0
	for _, sym := range symbols {
		if _, err := h.calcOne(sym); err != nil {
			log.Printf("[chip-score] %s 計算失敗: %v", sym, err)
		} else {
			done++
		}
	}
	log.Printf("[chip-score] 完成 %d/%d 支", done, len(symbols))
	return nil
}

// ── 評分結構 ─────────────────────────────────────────────────────────────────
type breakdownData struct {
	Institutional struct {
		ForeignNet5d float32 `json:"foreign_net_5d"` // 近5日外資淨買超（張）
		TrustNet5d   float32 `json:"trust_net_5d"`
		DealerNet5d  float32 `json:"dealer_net_5d"`
		Total5d      float32 `json:"total_5d"`
		NormScore    float32 `json:"norm_score"`
	} `json:"institutional"`
	Major struct {
		NetVol5d  float32 `json:"net_vol_5d"` // 近5日主力前10名合計淨買超（張）
		NormScore float32 `json:"norm_score"`
	} `json:"major"`
	ChipsPyramid struct {
		LargeHolderPct    float32 `json:"large_holder_pct"`     // 最新大戶持股%
		LargeHolderTrend  float32 `json:"large_holder_trend"`   // 與上期差值（正=上升）
		NormScore         float32 `json:"norm_score"`
	} `json:"chips_pyramid"`
	Winrate struct {
		AvgWinRatePct float32 `json:"avg_win_rate_pct"` // 近期買超券商平均勝率%
		NormScore     float32 `json:"norm_score"`
	} `json:"winrate"`
}

// calcOne 計算單支股票的籌碼評分並 UPSERT 到 DB
func (h *ChipScoreHandler) calcOne(symbol string) (*models.StockChipScore, error) {
	today := time.Now()
	bd := breakdownData{}

	// ── 1. 三大法人面（35 分）──────────────────────────────────────────────
	{
		type instRow struct {
			ForeignNet float32
			TrustNet   float32
			DealerNet  float32
		}
		var rows []instRow
		h.db.Raw(`
			SELECT foreign_net/1000 AS foreign_net,
			       trust_net/1000   AS trust_net,
			       dealer_net/1000  AS dealer_net
			FROM institutional_trading
			WHERE symbol = ? AND date >= NOW() - INTERVAL '7 days'
			ORDER BY date DESC LIMIT 5`, symbol).Scan(&rows)

		var sumForeign, sumTrust, sumDealer float32
		for _, r := range rows {
			sumForeign += r.ForeignNet
			sumTrust += r.TrustNet
			sumDealer += r.DealerNet
		}
		bd.Institutional.ForeignNet5d = sumForeign
		bd.Institutional.TrustNet5d = sumTrust
		bd.Institutional.DealerNet5d = sumDealer
		bd.Institutional.Total5d = sumForeign + sumTrust + sumDealer

		// 正規化：以 ±5000 張為極端值，映射到 0~35
		total := bd.Institutional.Total5d
		clamped := clamp32(total, -5000, 5000)
		bd.Institutional.NormScore = (clamped/5000)*17.5 + 17.5 // 0~35
	}

	// ── 2. 主力券商面（35 分）────────────────────────────────────────────────
	{
		type majorRow struct{ NetVol int }
		var rows []majorRow
		// 取最新資料日的1日數據，計算前10名合計淨買超
		h.db.Raw(`
			SELECT SUM(net_vol) AS net_vol
			FROM major_broker_records
			WHERE symbol = ? AND days = 1
			  AND data_date = (
			    SELECT MAX(data_date) FROM major_broker_records
			    WHERE symbol = ? AND days = 1
			  )`, symbol, symbol).Scan(&rows)

		var netVol float32
		if len(rows) > 0 {
			netVol = float32(rows[0].NetVol)
		}
		bd.Major.NetVol5d = netVol

		// 正規化：以 ±2000 張為極端值，映射到 0~35
		clamped := clamp32(netVol, -2000, 2000)
		bd.Major.NormScore = (clamped/2000)*17.5 + 17.5
	}

	// ── 3. 大戶持股面（15 分）────────────────────────────────────────────────
	{
		type pyramidRow struct {
			SharePct     float32
			DataDate     time.Time
		}
		// 取最近兩筆籌碼金字塔快照，計算大戶（前5層 = tier_rank > len-5）的股數佔比
		var snapshots []uint
		h.db.Raw(`
			SELECT id FROM chips_holder_snapshots
			WHERE symbol = ? ORDER BY data_date DESC LIMIT 2`, symbol).Pluck("id", &snapshots)

		getTopSharePct := func(snapshotID uint) float32 {
			var totalPct float32
			h.db.Raw(`
				SELECT COALESCE(SUM(share_pct), 0) AS share_pct
				FROM chips_holder_distributions
				WHERE snapshot_id = ?
				  AND tier_rank >= (
				    SELECT MAX(tier_rank) - 4 FROM chips_holder_distributions
				    WHERE snapshot_id = ?
				  )`, snapshotID, snapshotID).Scan(&totalPct)
			return totalPct
		}

		if len(snapshots) >= 1 {
			bd.ChipsPyramid.LargeHolderPct = getTopSharePct(snapshots[0])
		}
		if len(snapshots) >= 2 {
			prev := getTopSharePct(snapshots[1])
			bd.ChipsPyramid.LargeHolderTrend = bd.ChipsPyramid.LargeHolderPct - prev
		}

		// 正規化：大戶持股 >30% 滿分，趨勢加成
		base := clamp32(bd.ChipsPyramid.LargeHolderPct/30.0*10, 0, 10)
		trendBonus := clamp32(bd.ChipsPyramid.LargeHolderTrend*2, -5, 5)
		bd.ChipsPyramid.NormScore = clamp32(base+trendBonus, 0, 15)
	}

	// ── 4. 勝率面（15 分）───────────────────────────────────────────────────
	{
		// 找最新買超榜上的券商，取其歷史平均勝率
		type brokerRow struct{ BrokerName string }
		var brokers []brokerRow
		h.db.Raw(`
			SELECT DISTINCT broker_name
			FROM major_broker_records
			WHERE symbol = ? AND side = 'buy' AND days = 1
			  AND data_date = (
			    SELECT MAX(data_date) FROM major_broker_records
			    WHERE symbol = ? AND side = 'buy' AND days = 1
			  )`, symbol, symbol).Scan(&brokers)

		var totalWR float32
		count := 0
		for _, b := range brokers {
			var wr float32
			h.db.Raw(`
				SELECT COALESCE(
				  100.0 * COUNT(*) FILTER (WHERE is_win = true) / NULLIF(COUNT(*) FILTER (WHERE is_win IS NOT NULL), 0),
				  0
				) AS wr
				FROM broker_trade_events
				WHERE symbol = ? AND broker_name = ?`, symbol, b.BrokerName).Scan(&wr)
			totalWR += wr
			count++
		}
		if count > 0 {
			bd.Winrate.AvgWinRatePct = totalWR / float32(count)
		}
		// 正規化：60%勝率滿分，50%~60% 線性映射到 7.5~15
		bd.Winrate.NormScore = clamp32(bd.Winrate.AvgWinRatePct/100.0*15, 0, 15)
	}

	// ── 合計 ─────────────────────────────────────────────────────────────────
	totalScore := bd.Institutional.NormScore + bd.Major.NormScore +
		bd.ChipsPyramid.NormScore + bd.Winrate.NormScore

	bdJSON, _ := json.Marshal(bd)

	score := models.StockChipScore{
		Symbol:             symbol,
		CalcDate:           today,
		TotalScore:         float64(totalScore),
		InstitutionalScore: float64(bd.Institutional.NormScore),
		MajorScore:         float64(bd.Major.NormScore),
		ChipsPyramidScore:  float64(bd.ChipsPyramid.NormScore),
		WinrateScore:       float64(bd.Winrate.NormScore),
		Breakdown:          string(bdJSON),
	}

	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"calc_date", "total_score", "institutional_score",
			"major_score", "chips_pyramid_score", "winrate_score",
			"breakdown", "updated_at",
		}),
	}).Create(&score).Error; err != nil {
		return nil, fmt.Errorf("upsert chip score: %w", err)
	}
	return &score, nil
}

func clamp32(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
