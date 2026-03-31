package winrate

import (
	"time"

	"stock-backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultMaxHoldDays = 120
const defaultMinNetVol = 100

// CalculateSymbol 計算單一股票所有券商的「建倉→出場」事件配對，並 UPSERT 至 broker_trade_events。
// 回傳寫入筆數或錯誤。
func CalculateSymbol(db *gorm.DB, symbol string, maxHoldDays, minNetVol int) (int, error) {
	if maxHoldDays <= 0 {
		maxHoldDays = defaultMaxHoldDays
	}
	if minNetVol <= 0 {
		minNetVol = defaultMinNetVol
	}

	// 1. 抓取所有買超（建倉）事件，JOIN daily_prices 取得當日收盤價
	type entryRow struct {
		BrokerName  string    `gorm:"column:broker_name"`
		EntryDate   time.Time `gorm:"column:entry_date"`
		EntryClose  float64   `gorm:"column:entry_close"`
		EntryNetVol int       `gorm:"column:entry_net_vol"`
	}
	var entries []entryRow
	if err := db.Raw(`
		SELECT mbr.broker_name,
		       mbr.data_date::date AS entry_date,
		       dp.close            AS entry_close,
		       mbr.net_vol         AS entry_net_vol
		FROM major_broker_records mbr
		JOIN daily_prices dp
		  ON dp.symbol       = mbr.symbol
		 AND dp.date::date   = mbr.data_date::date
		WHERE mbr.symbol  = ?
		  AND mbr.side    = 'buy'
		  AND mbr.days    = 1
		  AND mbr.net_vol >= ?
		ORDER BY mbr.broker_name, mbr.data_date
	`, symbol, minNetVol).Scan(&entries).Error; err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}

	// 2. 抓取所有賣超（出場）事件
	type exitRow struct {
		BrokerName string    `gorm:"column:broker_name"`
		ExitDate   time.Time `gorm:"column:exit_date"`
		ExitClose  float64   `gorm:"column:exit_close"`
		ExitNetVol int       `gorm:"column:exit_net_vol"`
	}
	var exits []exitRow
	if err := db.Raw(`
		SELECT mbr.broker_name,
		       mbr.data_date::date AS exit_date,
		       dp.close            AS exit_close,
		       ABS(mbr.net_vol)    AS exit_net_vol
		FROM major_broker_records mbr
		JOIN daily_prices dp
		  ON dp.symbol       = mbr.symbol
		 AND dp.date::date   = mbr.data_date::date
		WHERE mbr.symbol = ?
		  AND mbr.side   = 'sell'
		  AND mbr.days   = 1
		ORDER BY mbr.broker_name, mbr.data_date
	`, symbol).Scan(&exits).Error; err != nil {
		return 0, err
	}

	// 3. 建立 broker → sorted exit events 的索引（已按 data_date 升序）
	exitByBroker := make(map[string][]exitRow, len(exits))
	for _, e := range exits {
		exitByBroker[e.BrokerName] = append(exitByBroker[e.BrokerName], e)
	}

	// 4. 逐筆建倉事件，搜尋最近的出場信號
	maxHoldDur := time.Duration(maxHoldDays) * 24 * time.Hour
	events := make([]models.BrokerTradeEvent, 0, len(entries))

	for _, entry := range entries {
		evt := models.BrokerTradeEvent{
			Symbol:      symbol,
			BrokerName:  entry.BrokerName,
			EntryDate:   entry.EntryDate,
			EntryClose:  entry.EntryClose,
			EntryNetVol: entry.EntryNetVol,
		}

		deadline := entry.EntryDate.Add(maxHoldDur)
		for _, ex := range exitByBroker[entry.BrokerName] {
			if ex.ExitDate.After(entry.EntryDate) && !ex.ExitDate.After(deadline) {
				d := ex.ExitDate
				cl := ex.ExitClose
				nv := ex.ExitNetVol
				ret := (cl - entry.EntryClose) / entry.EntryClose * 100
				hd := int(d.Sub(entry.EntryDate).Hours() / 24)
				win := cl > entry.EntryClose

				evt.ExitDate = &d
				evt.ExitClose = &cl
				evt.ExitNetVol = &nv
				evt.ReturnPct = &ret
				evt.HoldingDays = &hd
				evt.IsWin = &win
				break
			}
		}

		events = append(events, evt)
	}

	// 5. UPSERT：更新出場欄位，但已有出場紀錄者不更新
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "symbol"}, {Name: "broker_name"}, {Name: "entry_date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"exit_date", "exit_close", "exit_net_vol",
			"return_pct", "holding_days", "is_win", "updated_at",
		}),
	}).CreateInBatches(events, 500)

	return len(events), result.Error
}
