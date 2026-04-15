package backtest
// backtest/calculator.go — 回測計算引擎

import (
	"math"
	"sort"
	"time"

	"stock-backend/internal/models"

	"gorm.io/gorm"
)

// StrategyParams 回測策略參數
type StrategyParams struct {
	// 訊號條件（技術指標）
	EntryMAShort int     `json:"entry_ma_short"` // 短均線週期（預設 5）
	EntryMALong  int     `json:"entry_ma_long"`  // 長均線週期（預設 20）
	ExitMAShort  int     `json:"exit_ma_short"`  // 出場短均線（預設 5）
	ExitMALong   int     `json:"exit_ma_long"`   // 出場長均線（預設 20）

	// 資金管理
	CapitalPerTrade float64 `json:"capital_per_trade"` // 每次進場用幾%資金（0~1）
	MaxPositions    int     `json:"max_positions"`     // 最多同時持倉幾支

	// 風控
	StopLossPct   float64 `json:"stop_loss_pct"`   // 停損 % (負值, e.g. -0.07)
	TakeProfitPct float64 `json:"take_profit_pct"` // 停利 % (正值, e.g. 0.15)
	MaxHoldDays   int     `json:"max_hold_days"`   // 最長持倉天數

	// 費用
	FeeRate   float64 `json:"fee_rate"`   // 手續費率（預設 0.001425）
	TaxRate   float64 `json:"tax_rate"`   // 成交稅率（預設 0.003，賣方）
}

// DefaultParams 回傳預設策略參數
func DefaultParams() StrategyParams {
	return StrategyParams{
		EntryMAShort:    5,
		EntryMALong:     20,
		ExitMAShort:     5,
		ExitMALong:      20,
		CapitalPerTrade: 0.1,
		MaxPositions:    5,
		StopLossPct:     -0.07,
		TakeProfitPct:   0.15,
		MaxHoldDays:     60,
		FeeRate:         0.001425,
		TaxRate:         0.003,
	}
}

// Result 回測結果
type Result struct {
	Trades       []models.BacktestTrade
	EquityCurve  []models.BacktestEquityPoint
	TotalReturn  float64
	AnnualReturn float64
	MaxDrawdown  float64
	WinRate      float64
	SharpeRatio  float64
	TotalTrades  int
}

// Run 執行單股票回測
// symbol="" 表示全市場（不建議直接呼叫，由 handler 批次呼叫）
func Run(
	db *gorm.DB,
	jobID uint,
	symbol string,
	startDate, endDate string,
	initialCapital float64,
	p StrategyParams,
) Result {
	// 取日K資料
	var prices []models.DailyPrice
	q := db.Where("symbol = ? AND date >= ? AND date <= ?", symbol, startDate, endDate).
		Order("date ASC").
		Find(&prices)
	if q.Error != nil || len(prices) < p.EntryMALong+5 {
		return Result{}
	}

	closes := make([]float64, len(prices))
	for i, pr := range prices {
		closes[i] = pr.Close
	}

	maShort := calcMA(closes, p.EntryMAShort)
	maLong  := calcMA(closes, p.EntryMALong)

	capital := initialCapital
	var trades []models.BacktestTrade
	var equityCurve []models.BacktestEquityPoint

	type Position struct {
		EntryIdx   int
		EntryPrice float64
		Shares     int64
		EntryDate  string
	}
	var positions []Position

	peakEquity := capital
	maxDD := 0.0

	for i := p.EntryMALong; i < len(prices); i++ {
		pr := prices[i]
		dateStr := pr.Date.Format("2006-01-02")
		price := pr.Close

		if maShort[i] == nil || maLong[i] == nil {
			continue
		}
		ms := *maShort[i]
		ml := *maLong[i]

		// ── 出場檢查（先檢查，避免同日進出） ──
		var remaining []Position
		for _, pos := range positions {
			entryD, _ := time.Parse("2006-01-02", pos.EntryDate)
			holdDays  := int(pr.Date.Sub(entryD).Hours() / 24)
			pnlPct    := (price - pos.EntryPrice) / pos.EntryPrice

			var reason string
			switch {
			case pnlPct <= p.StopLossPct:
				reason = "stop_loss"
			case pnlPct >= p.TakeProfitPct:
				reason = "take_profit"
			case holdDays >= p.MaxHoldDays:
				reason = "max_hold"
			case i > 0 && maShort[i-1] != nil && maLong[i-1] != nil:
				// 死叉出場
				prevMs := *maShort[i-1]
				prevMl := *maLong[i-1]
				if prevMs >= prevMl && ms < ml {
					reason = "signal"
				}
			}

			if reason != "" {
				cost := float64(pos.Shares) * price
				tax  := cost * p.TaxRate
				fee  := cost * p.FeeRate
				recv := cost - tax - fee
				pnl  := recv - float64(pos.Shares)*pos.EntryPrice
				capital += recv
				trades = append(trades, models.BacktestTrade{
					JobID:      jobID,
					Symbol:     symbol,
					EntryDate:  pos.EntryDate,
					ExitDate:   dateStr,
					EntryPrice: pos.EntryPrice,
					ExitPrice:  price,
					Shares:     pos.Shares,
					PnL:        math.Round(pnl*100) / 100,
					PnLPct:     math.Round(pnlPct*10000) / 100,
					HoldDays:   holdDays,
					ExitReason: reason,
				})
			} else {
				remaining = append(remaining, pos)
			}
		}
		positions = remaining

		// ── 進場（黃金交叉） ──
		if i > 0 && maShort[i-1] != nil && maLong[i-1] != nil {
			prevMs := *maShort[i-1]
			prevMl := *maLong[i-1]
			if prevMs < prevMl && ms >= ml && len(positions) < p.MaxPositions {
				// 可動用資金
				budget  := capital * p.CapitalPerTrade
				cost1   := price * (1 + p.FeeRate)
				shares  := int64(budget / cost1 / 1000) * 1000 // 取整張（1張=1000股）
				if shares > 0 {
					spent := float64(shares)*price + float64(shares)*price*p.FeeRate
					if spent <= capital {
						capital -= spent
						positions = append(positions, Position{
							EntryIdx:   i,
							EntryPrice: price,
							Shares:     shares,
							EntryDate:  dateStr,
						})
					}
				}
			}
		}

		// ── 計算當日總資產 ──
		equity := capital
		for _, pos := range positions {
			equity += float64(pos.Shares) * price
		}
		equityCurve = append(equityCurve, models.BacktestEquityPoint{
			JobID:  jobID,
			Date:   dateStr,
			Equity: math.Round(equity*100) / 100,
			Cash:   math.Round(capital*100) / 100,
		})
		if equity > peakEquity {
			peakEquity = equity
		}
		dd := (peakEquity - equity) / peakEquity
		if dd > maxDD {
			maxDD = dd
		}
	}

	// 強制平倉剩餘倉位（以最後一日收盤）
	if len(prices) > 0 {
		lastPr := prices[len(prices)-1]
		lastPrice := lastPr.Close
		lastDate  := lastPr.Date.Format("2006-01-02")
		for _, pos := range positions {
			entryD, _ := time.Parse("2006-01-02", pos.EntryDate)
			holdDays  := int(lastPr.Date.Sub(entryD).Hours() / 24)
			pnlPct    := (lastPrice - pos.EntryPrice) / pos.EntryPrice
			cost      := float64(pos.Shares) * lastPrice
			pnl       := cost*(1-lastPr.Close*0+1) - float64(pos.Shares)*lastPrice*(1+p.FeeRate) // simplified
			pnl        = float64(pos.Shares)*(lastPrice-pos.EntryPrice) - float64(pos.Shares)*lastPrice*(p.FeeRate+p.TaxRate)
			trades = append(trades, models.BacktestTrade{
				JobID:      jobID,
				Symbol:     symbol,
				EntryDate:  pos.EntryDate,
				ExitDate:   lastDate,
				EntryPrice: pos.EntryPrice,
				ExitPrice:  lastPrice,
				Shares:     pos.Shares,
				PnL:        math.Round(pnl*100) / 100,
				PnLPct:     math.Round(pnlPct*10000) / 100,
				HoldDays:   holdDays,
				ExitReason: "end_date",
			})
		}
	}

	// 統計
	wins := 0
	for _, t := range trades {
		if t.PnL > 0 { wins++ }
	}
	totalTrades := len(trades)
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
	}

	finalEquity := initialCapital
	if len(equityCurve) > 0 {
		finalEquity = equityCurve[len(equityCurve)-1].Equity
	}
	totalReturn := (finalEquity - initialCapital) / initialCapital * 100

	// 年化報酬率
	annualReturn := 0.0
	if len(equityCurve) >= 2 {
		start, _ := time.Parse("2006-01-02", equityCurve[0].Date)
		end, _   := time.Parse("2006-01-02", equityCurve[len(equityCurve)-1].Date)
		years    := end.Sub(start).Hours() / 24 / 365
		if years > 0 {
			annualReturn = (math.Pow(finalEquity/initialCapital, 1/years) - 1) * 100
		}
	}

	// Sharpe Ratio（簡化，無風險利率假設 0）
	sharpe := calcSharpe(equityCurve)

	// 整理交易順序
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].EntryDate < trades[j].EntryDate
	})

	return Result{
		Trades:      trades,
		EquityCurve: equityCurve,
		TotalReturn:  math.Round(totalReturn*100) / 100,
		AnnualReturn: math.Round(annualReturn*100) / 100,
		MaxDrawdown:  math.Round(maxDD*10000) / 100,
		WinRate:      math.Round(winRate*100) / 100,
		SharpeRatio:  math.Round(sharpe*100) / 100,
		TotalTrades:  totalTrades,
	}
}

// calcMA 計算簡單均線，回傳指標陣列（前 period-1 個為 nil）
func calcMA(data []float64, period int) []*float64 {
	result := make([]*float64, len(data))
	sum := 0.0
	for i, v := range data {
		sum += v
		if i >= period {
			sum -= data[i-period]
		}
		if i >= period-1 {
			avg := sum / float64(period)
			result[i] = &avg
		}
	}
	return result
}

// calcSharpe 計算日報酬率的 Sharpe（無風險利率=0，年化）
func calcSharpe(curve []models.BacktestEquityPoint) float64 {
	if len(curve) < 2 { return 0 }
	var dailyRet []float64
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Equity
		curr := curve[i].Equity
		if prev > 0 {
			dailyRet = append(dailyRet, (curr-prev)/prev)
		}
	}
	if len(dailyRet) == 0 { return 0 }
	// 平均
	mean := 0.0
	for _, r := range dailyRet { mean += r }
	mean /= float64(len(dailyRet))
	// 標準差
	variance := 0.0
	for _, r := range dailyRet { variance += (r - mean) * (r - mean) }
	variance /= float64(len(dailyRet))
	std := math.Sqrt(variance)
	if std == 0 { return 0 }
	return mean / std * math.Sqrt(252) // 年化
}
