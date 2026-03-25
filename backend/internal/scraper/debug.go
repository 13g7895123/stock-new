package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ─────────────────────────────────────────────
// 型別定義
// ─────────────────────────────────────────────

type FilterRule struct {
	Name    string `json:"name"`
	Rule    string `json:"rule"`
	Applied string `json:"applied"`
}

type DebugRow struct {
	RowNum     int      `json:"row_num"`
	Raw        []string `json:"raw"`
	Date       string   `json:"date,omitempty"`
	Open       float64  `json:"open"`
	High       float64  `json:"high"`
	Low        float64  `json:"low"`
	Close      float64  `json:"close"`
	Volume     int64    `json:"volume"`
	TxValue    int64    `json:"tx_value"`
	TxCount    int      `json:"tx_count"`
	Skipped    bool     `json:"skipped"`
	SkipReason string   `json:"skip_reason,omitempty"`
}

type DebugRawMonth struct {
	Symbol      string       `json:"symbol"`
	Market      string       `json:"market"`
	YearMonth   string       `json:"year_month"`
	URL         string       `json:"url"`
	Fields      []string     `json:"fields"`
	RawCount    int          `json:"raw_count"`
	PassCount   int          `json:"pass_count"`
	SkipCount   int          `json:"skip_count"`
	SkipReasons []string     `json:"skip_reasons"`
	Rows        []DebugRow   `json:"rows"`
	FilterRules []FilterRule `json:"filter_rules"`
}

// FetchDebugRawMonth 拉取單月資料並回傳每列的解析 / 過濾細節
func FetchDebugRawMonth(symbol, market, yyyymm string) (*DebugRawMonth, error) {
	if market == "TWSE" {
		return fetchDebugTWSE(symbol, yyyymm)
	}
	return fetchDebugTPEX(symbol, yyyymm)
}

// ─────────────────────────────────────────────
// TWSE
// ─────────────────────────────────────────────

func fetchDebugTWSE(symbol, yyyymm string) (*DebugRawMonth, error) {
	url := fmt.Sprintf("%s?response=json&date=%s01&stockNo=%s", TWSEStockDayURL, yyyymm, symbol)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	var r twseStockDayResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if r.Stat != "OK" {
		return nil, fmt.Errorf("twse stat: %s", r.Stat)
	}

	result := &DebugRawMonth{
		Symbol:    symbol,
		Market:    "TWSE",
		YearMonth: yyyymm,
		URL:       url,
		Fields:    r.Fields,
		RawCount:  len(r.Data),
		FilterRules: []FilterRule{
			{
				Name:    "欄位數量",
				Rule:    `len(row) < 9`,
				Applied: "原始列欄位不足 9 個直接跳過（不完整資料）",
			},
			{
				Name:    "日期格式轉換",
				Rule:    `parseROCDate(row[0])：民國年/MM/DD → 西元年`,
				Applied: "民國年 + 1911 = 西元年，格式錯誤則跳過",
			},
			{
				Name:    "停牌 / 無效日",
				Rule:    `open == 0 && high == 0`,
				Applied: "開盤與最高同時為 0，視為停牌或節假日，跳過",
			},
			{
				Name:    "千分位清理",
				Rule:    `cleanNumber: remove "," and whitespace`,
				Applied: "所有數字欄位先去除逗號與空白再解析",
			},
			{
				Name:    "非數字特殊值",
				Rule:    `"--" / "" / "N/A" → 0`,
				Applied: "除權息日等特殊格式以 0 代替，不跳過整列",
			},
		},
	}

	skipReasonMap := map[string]int{}
	for i, row := range r.Data {
		dr := DebugRow{RowNum: i + 1, Raw: row}

		if len(row) < 9 {
			dr.Skipped = true
			dr.SkipReason = "欄位數不足 9"
			skipReasonMap["欄位數不足 9"]++
			result.Rows = append(result.Rows, dr)
			continue
		}

		date, err := parseROCDate(row[0])
		if err != nil {
			dr.Skipped = true
			dr.SkipReason = fmt.Sprintf("日期格式錯誤: %q", row[0])
			skipReasonMap["日期格式錯誤"]++
			result.Rows = append(result.Rows, dr)
			continue
		}
		dr.Date = date.Format("2006-01-02")

		open, _ := parsePrice(row[3])
		high, _ := parsePrice(row[4])
		low, _ := parsePrice(row[5])
		close_, _ := parsePrice(row[6])
		vol, _ := parseVolume(row[1])
		txVal, _ := parseVolume(row[2])
		txCnt, _ := strconv.Atoi(cleanNumber(row[8]))

		dr.Open = open
		dr.High = high
		dr.Low = low
		dr.Close = close_
		dr.Volume = vol
		dr.TxValue = txVal
		dr.TxCount = txCnt

		if open == 0 && high == 0 {
			dr.Skipped = true
			dr.SkipReason = "開盤=0 且 最高=0（停牌/除權）"
			skipReasonMap["停牌/除權"]++
		}

		result.Rows = append(result.Rows, dr)
	}

	skipCount := 0
	for reason, count := range skipReasonMap {
		skipCount += count
		result.SkipReasons = append(result.SkipReasons, fmt.Sprintf("%s × %d", reason, count))
	}
	result.SkipCount = skipCount
	result.PassCount = result.RawCount - skipCount

	return result, nil
}

// ─────────────────────────────────────────────
// TPEX
// ─────────────────────────────────────────────

func fetchDebugTPEX(symbol, yyyymm string) (*DebugRawMonth, error) {
	if len(yyyymm) < 6 {
		return nil, fmt.Errorf("invalid yyyymm: %s", yyyymm)
	}
	year, _ := strconv.Atoi(yyyymm[:4])
	rocYear := year - 1911
	rocDate := fmt.Sprintf("%d/%s", rocYear, yyyymm[4:6])
	url := fmt.Sprintf("%s?l=zh-tw&d=%s&stkno=%s", TPEXStockDayURL, rocDate, symbol)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	var r tpexStockDayResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// TPEX 欄位固定順序（文件 / 實測）
	fields := []string{"日期", "股名", "成交股數", "成交金額", "開盤", "最高", "最低", "收盤", "漲跌", "成交筆數"}

	result := &DebugRawMonth{
		Symbol:    symbol,
		Market:    "TPEX",
		YearMonth: yyyymm,
		URL:       url,
		Fields:    fields,
		RawCount:  len(r.AaData),
		FilterRules: []FilterRule{
			{
				Name:    "欄位數量",
				Rule:    `len(row) < 10`,
				Applied: "原始列欄位不足 10 個直接跳過",
			},
			{
				Name:    "日期格式轉換",
				Rule:    `parseROCDate(row[0])：民國年/MM/DD → 西元年`,
				Applied: "民國年 + 1911 = 西元年，格式錯誤則跳過",
			},
			{
				Name:    "停牌 / 無效日",
				Rule:    `open == 0 && high == 0`,
				Applied: "開盤與最高同時為 0，視為停牌或節假日，跳過",
			},
			{
				Name:    "日期查詢方式",
				Rule:    `西元年 YYYYMM → 民國年 YYY/MM`,
				Applied: "TPEX API 使用民國年格式，需事先轉換查詢參數",
			},
			{
				Name:    "千分位清理 & 特殊值",
				Rule:    `cleanNumber + parsePrice("--" → 0)`,
				Applied: "同 TWSE",
			},
		},
	}

	skipReasonMap := map[string]int{}
	for i, row := range r.AaData {
		dr := DebugRow{RowNum: i + 1, Raw: row}

		if len(row) < 10 {
			dr.Skipped = true
			dr.SkipReason = "欄位數不足 10"
			skipReasonMap["欄位數不足 10"]++
			result.Rows = append(result.Rows, dr)
			continue
		}

		date, err := parseROCDate(row[0])
		if err != nil {
			dr.Skipped = true
			dr.SkipReason = fmt.Sprintf("日期格式錯誤: %q", row[0])
			skipReasonMap["日期格式錯誤"]++
			result.Rows = append(result.Rows, dr)
			continue
		}
		dr.Date = date.Format("2006-01-02")

		open, _ := parsePrice(row[4])
		high, _ := parsePrice(row[5])
		low, _ := parsePrice(row[6])
		close_, _ := parsePrice(row[7])
		vol, _ := parseVolume(row[2])
		txVal, _ := parseVolume(row[3])
		txCnt, _ := strconv.Atoi(cleanNumber(row[9]))

		dr.Open = open
		dr.High = high
		dr.Low = low
		dr.Close = close_
		dr.Volume = vol
		dr.TxValue = txVal
		dr.TxCount = txCnt

		if open == 0 && high == 0 {
			dr.Skipped = true
			dr.SkipReason = "開盤=0 且 最高=0（停牌/除權）"
			skipReasonMap["停牌/除權"]++
		}

		result.Rows = append(result.Rows, dr)
	}

	skipCount := 0
	for reason, count := range skipReasonMap {
		skipCount += count
		result.SkipReasons = append(result.SkipReasons, fmt.Sprintf("%s × %d", reason, count))
	}
	result.SkipCount = skipCount
	result.PassCount = result.RawCount - skipCount

	return result, nil
}
