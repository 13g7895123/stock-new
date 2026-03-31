package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"
)

// ─────────────────────────────────────────────
// TWSE：全市場最新一日（STOCK_DAY_ALL）
// ─────────────────────────────────────────────

const TWSEDayAllURL = "https://openapi.twse.com.tw/v1/exchangeReport/STOCK_DAY_ALL"

type twseDayAllRecord struct {
	Code         string `json:"Code"`
	TradeVolume  string `json:"TradeVolume"`
	TradeValue   string `json:"TradeValue"`
	OpeningPrice string `json:"OpeningPrice"`
	HighestPrice string `json:"HighestPrice"`
	LowestPrice  string `json:"LowestPrice"`
	ClosingPrice string `json:"ClosingPrice"`
	Transaction  string `json:"Transaction"`
}

// FetchTWSEDayAll 抓取 TWSE 全市場最新一日 OHLCV，date 為資料所屬日期（呼叫方傳入）
func FetchTWSEDayAll(date time.Time) ([]models.DailyPrice, error) {
	req, err := http.NewRequest(http.MethodGet, TWSEDayAllURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var records []twseDayAllRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	result := make([]models.DailyPrice, 0, len(records))
	for _, r := range records {
		if !regularStockPattern.MatchString(r.Code) {
			continue
		}
		open, _ := parsePrice(r.OpeningPrice)
		high, _ := parsePrice(r.HighestPrice)
		low, _ := parsePrice(r.LowestPrice)
		close_, _ := parsePrice(r.ClosingPrice)
		vol, _ := parseVolume(r.TradeVolume)
		txVal, _ := parseVolume(r.TradeValue)
		txCnt, _ := strconv.Atoi(cleanNumber(r.Transaction))

		// 跳過無效資料（開盤 0 通常表示當日未成交 / 停牌）
		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol:  r.Code,
			Date:    date,
			Open:    open,
			High:    high,
			Low:     low,
			Close:   close_,
			Volume:  vol,
			TxValue: txVal,
			TxCount: txCnt,
		})
	}
	return result, nil
}

// ─────────────────────────────────────────────
// TWSE：單支股票歷史月資料（STOCK_DAY）
// ─────────────────────────────────────────────

const TWSEStockDayURL = "https://www.twse.com.tw/exchangeReport/STOCK_DAY"

type twseStockDayResp struct {
	Stat   string     `json:"stat"`
	Fields []string   `json:"fields"`
	Data   [][]string `json:"data"`
}

// FetchTWSEStockHistory 抓取單支股票指定年月的日K（月份格式：YYYYMM 或 YYYYMMDD）
// TWSE 以月為單位回傳，傳入任何該月日期皆可
func FetchTWSEStockHistory(symbol, yyyymm string) ([]models.DailyPrice, error) {
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

	// fields: [日期 成交股數 成交金額 開盤價 最高價 最低價 收盤價 漲跌價差 成交筆數]
	result := make([]models.DailyPrice, 0, len(r.Data))
	for _, row := range r.Data {
		if len(row) < 9 {
			continue
		}
		date, err := parseROCDate(row[0]) // "113/01/02"
		if err != nil {
			continue
		}
		vol, _ := parseVolume(row[1])
		txVal, _ := parseVolume(row[2])
		open, _ := parsePrice(row[3])
		high, _ := parsePrice(row[4])
		low, _ := parsePrice(row[5])
		close_, _ := parsePrice(row[6])
		txCnt, _ := strconv.Atoi(cleanNumber(row[8]))

		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol:  symbol,
			Date:    date,
			Open:    open,
			High:    high,
			Low:     low,
			Close:   close_,
			Volume:  vol,
			TxValue: txVal,
			TxCount: txCnt,
		})
	}
	return result, nil
}

// ─────────────────────────────────────────────
// TPEX：全市場最新一日（tpex_mainboard_quotes）
// ─────────────────────────────────────────────

type tpexDayRecord struct {
	Code    string `json:"SecuritiesCompanyCode"`
	Close   string `json:"Close"`
	Open    string `json:"Open"`
	High    string `json:"High"`
	Low     string `json:"Low"`
	Volume  string `json:"TradingShares"` // 張數（千股）
	TxValue string `json:"Amount"`
	TxCount string `json:"TransactionRecord"`
}

// ─────────────────────────────────────────────
// TPEX：單支股票歷史日K（daily_trading_info）
// ─────────────────────────────────────────────

const TPEXStockDayURL = "https://www.tpex.org.tw/web/stock/aftertrading/daily_trading_info/st43_result.php"

type tpexStockDayResp struct {
	TotalRecords int        `json:"iTotalRecords"`
	AaData       [][]string `json:"aaData"`
}

// FetchTPEXStockHistory 抓取單支上櫃股票指定年月的日K
// yyyymm 格式為 "YYYYMM"（如 "202501"）
func FetchTPEXStockHistory(symbol, yyyymm string) ([]models.DailyPrice, error) {
	// 轉換為民國年 YYY/MM
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

	// aaData 欄位順序：[日期, 股名, 成交股數, 成交金額, 開盤, 最高, 最低, 收盤, 漲跌, 成交筆數]
	result := make([]models.DailyPrice, 0, len(r.AaData))
	for _, row := range r.AaData {
		if len(row) < 10 {
			continue
		}
		date, err := parseROCDate(row[0])
		if err != nil {
			continue
		}
		open, _ := parsePrice(row[4])
		high, _ := parsePrice(row[5])
		low, _ := parsePrice(row[6])
		close_, _ := parsePrice(row[7])
		vol, _ := parseVolume(row[2])
		txVal, _ := parseVolume(row[3])
		txCnt, _ := strconv.Atoi(cleanNumber(row[9]))

		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol:  symbol,
			Date:    date,
			Open:    open,
			High:    high,
			Low:     low,
			Close:   close_,
			Volume:  vol,
			TxValue: txVal,
			TxCount: txCnt,
		})
	}
	return result, nil
}

// FetchTPEXDayAll 重用現有 tpex_mainboard_quotes 端點，補充 OHLCV 欄位
func FetchTPEXDayAll(date time.Time) ([]models.DailyPrice, error) {
	req, err := http.NewRequest(http.MethodGet, TPEXOtcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var records []tpexDayRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	result := make([]models.DailyPrice, 0, len(records))
	for _, r := range records {
		if !regularStockPattern.MatchString(r.Code) {
			continue
		}
		open, _ := parsePrice(r.Open)
		high, _ := parsePrice(r.High)
		low, _ := parsePrice(r.Low)
		close_, _ := parsePrice(r.Close)
		vol, _ := parseVolume(r.Volume)
		txVal, _ := parseVolume(r.TxValue)
		txCnt, _ := strconv.Atoi(cleanNumber(r.TxCount))

		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol:  r.Code,
			Date:    date,
			Open:    open,
			High:    high,
			Low:     low,
			Close:   close_,
			Volume:  vol,
			TxValue: txVal,
			TxCount: txCnt,
		})
	}
	return result, nil
}

// ─────────────────────────────────────────────
// 共用解析工具
// ─────────────────────────────────────────────

// cleanNumber 移除千分位逗號（ASCII 及全形）與前後空白
func cleanNumber(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")  // ASCII 千分位逗號
	s = strings.ReplaceAll(s, "，", "") // 全形千分位逗號（部分來源）
	return s
}

// parsePrice 解析含逗號的價格字串，遇到 "--" / "除權" 等非數字回傳 0
func parsePrice(s string) (float64, error) {
	clean := cleanNumber(s)
	if clean == "--" || clean == "" || clean == "N/A" {
		return 0, nil
	}
	return strconv.ParseFloat(clean, 64)
}

// parseVolume 解析含千分位逗號的整數量（成交量、成交金額）
// 優先以 ParseInt 解析；若帶小數點（如 "1234567.00"）則 fallback 至 ParseFloat 再截斷
func parseVolume(s string) (int64, error) {
	clean := cleanNumber(s)
	if clean == "--" || clean == "" {
		return 0, nil
	}
	if v, err := strconv.ParseInt(clean, 10, 64); err == nil {
		return v, nil
	}
	if f, err := strconv.ParseFloat(clean, 64); err == nil {
		return int64(f), nil
	}
	return 0, fmt.Errorf("parseVolume: cannot parse %q", s)
}

// parseROCDate 將民國日期（"113/01/02"）轉換為 time.Time
func parseROCDate(s string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date: %s", s)
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	// 民國年 + 1911 = 西元年
	gregorianYear := year + 1911
	dateStr := fmt.Sprintf("%d/%s/%s", gregorianYear, parts[1], parts[2])
	return time.Parse("2006/01/02", dateStr)
}
