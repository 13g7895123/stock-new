package scraper

// broker.go — 券商 MoneyDJ API 爬取（一次取全部歷史日K）
//
// 優勢：一個 HTTP 請求取得整支股票完整歷史，不需要逐月迴圈。
// 回應格式（空白分隔 6 個 section，每 section 內用逗號分隔）：
//
//   date1,date2,...  open1,open2,...  high1,high2,...  low1,low2,...  close1,close2,...  vol1,vol2,...
//
// 日期格式：YYYY/MM/DD（西元）或 YYY/MM/DD（民國），兩種均支援。
//
// 券商 URL 樣板（優先順序，第一個成功即採用）：
//   {base}/z/BCD/czkc1.djbcd?a={symbol}&b=A&c={symbol}&E=1&ver=5
//
// 若要新增券商，在 DefaultBrokerBaseURLs 中加上 base URL 即可。

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"
)

// DefaultBrokerBaseURLs — 多家券商 base URL，依序 failover
// 可透過環境變數 BROKER_BASE_URLS（逗號分隔）覆蓋
var DefaultBrokerBaseURLs = []string{
	"http://fubon-ebrokerdj.fbs.com.tw/",
	"http://jdata.yuanta.com.tw/",
	"http://newjust.masterlink.com.tw/",
	"https://sjmain.esunsec.com.tw/",
	"http://kgieworld.moneydj.com/",
	"http://justdata.moneydj.com/",
}

const brokerDataPath = "z/BCD/czkc1.djbcd"

// BrokerFetchResult 包含券商 fetch 的結果與診斷資訊
type BrokerFetchResult struct {
	Source  string             // 成功的券商 base URL
	URL     string             // 完整請求 URL
	Records []models.DailyPrice
	Tried   []string           // 嘗試過但失敗的 URL
}

// BuildBrokerURL 依 base URL 與股票代號建構完整請求 URL
func BuildBrokerURL(base, symbol string) string {
	base = strings.TrimRight(base, "/")
	return fmt.Sprintf("%s/%s?a=%s&b=A&c=%s&E=1&ver=5", base, brokerDataPath, symbol, symbol)
}

// FetchBrokerStockHistory 使用券商 API 一次取得全部歷史日K
// 依序嘗試 DefaultBrokerBaseURLs，返回第一個成功的結果
func FetchBrokerStockHistory(symbol string) (*BrokerFetchResult, error) {
	return FetchBrokerStockHistoryWith(symbol, DefaultBrokerBaseURLs)
}

// FetchBrokerStockHistoryWith 與 FetchBrokerStockHistory 相同，但允許指定 base URLs
func FetchBrokerStockHistoryWith(symbol string, baseURLs []string) (*BrokerFetchResult, error) {
	result := &BrokerFetchResult{}
	for _, base := range baseURLs {
		url := BuildBrokerURL(base, symbol)
		records, err := fetchBrokerURL(symbol, url)
		if err == nil && len(records) > 0 {
			result.Source = base
			result.URL = url
			result.Records = records
			return result, nil
		}
		result.Tried = append(result.Tried, fmt.Sprintf("%s: %v", url, err))
	}
	return result, fmt.Errorf("all brokers failed or returned empty data")
}

// ─────────────────────────────────────────────
// 內部實作
// ─────────────────────────────────────────────

func fetchBrokerURL(symbol, url string) ([]models.DailyPrice, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/plain, */*")
	req.Header.Set("Referer", "http://www.moneydj.com/")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseBrokerResponse(symbol, string(body))
}

// parseBrokerResponse 解析券商回應文字
// 主格式：空白分隔 6 個 section，每 section 逗號分隔值
// 退路格式：Tab 分隔欄位（每行一天）
func parseBrokerResponse(symbol, text string) ([]models.DailyPrice, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty response")
	}

	// 主格式：sections 以分白分隔
	sections := strings.Fields(text)
	if len(sections) >= 6 {
		records, err := parseSectionFormat(symbol, sections)
		if err == nil && len(records) > 0 {
			return records, nil
		}
	}

	// 退路格式：tab / 多空白分隔的逐行格式
	return parseLineFormat(symbol, text)
}

// parseSectionFormat 解析「空白分段 + 逗號值」格式
// sections[0]=dates, [1]=open, [2]=high, [3]=low, [4]=close, [5]=volume
func parseSectionFormat(symbol string, sections []string) ([]models.DailyPrice, error) {
	dates := strings.Split(sections[0], ",")
	opens := strings.Split(sections[1], ",")
	highs := strings.Split(sections[2], ",")
	lows := strings.Split(sections[3], ",")
	closes := strings.Split(sections[4], ",")
	vols := strings.Split(sections[5], ",")

	n := len(dates)
	if n == 0 {
		return nil, fmt.Errorf("empty dates")
	}
	if len(opens) != n || len(highs) != n || len(lows) != n || len(closes) != n || len(vols) != n {
		return nil, fmt.Errorf("array length mismatch: dates=%d opens=%d", n, len(opens))
	}

	result := make([]models.DailyPrice, 0, n)
	for i := 0; i < n; i++ {
		date, err := parseBrokerDate(dates[i])
		if err != nil {
			continue
		}
		open, _ := parsePrice(cleanNumber(opens[i]))
		high, _ := parsePrice(cleanNumber(highs[i]))
		low, _ := parsePrice(cleanNumber(lows[i]))
		close_, _ := parsePrice(cleanNumber(closes[i]))

		var vol int64
		if v, err := strconv.ParseInt(cleanNumber(vols[i]), 10, 64); err == nil {
			vol = v
		}

		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol: symbol,
			Date:   date,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close_,
			Volume: vol,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid records after parsing sections")
	}
	return result, nil
}

// parseLineFormat 退路解析：逐行 tab/空白分隔
// 欄位順序：日期 開盤 最高 最低 收盤 成交量
func parseLineFormat(symbol, text string) ([]models.DailyPrice, error) {
	lines := strings.Split(text, "\n")
	result := make([]models.DailyPrice, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		date, err := parseBrokerDate(fields[0])
		if err != nil {
			continue
		}
		open, _ := parsePrice(cleanNumber(fields[1]))
		high, _ := parsePrice(cleanNumber(fields[2]))
		low, _ := parsePrice(cleanNumber(fields[3]))
		close_, _ := parsePrice(cleanNumber(fields[4]))

		var vol int64
		if v, err := strconv.ParseInt(cleanNumber(fields[5]), 10, 64); err == nil {
			vol = v
		}

		if open == 0 && high == 0 {
			continue
		}

		result = append(result, models.DailyPrice{
			Symbol: symbol,
			Date:   date,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close_,
			Volume: vol,
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid records in line format")
	}
	return result, nil
}

// parseBrokerDate 支援 YYYY/MM/DD（西元）與 YYY/MM/DD（民國）
func parseBrokerDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "/") // 部分券商用 "-" 分隔
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date: %s", s)
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year: %s", parts[0])
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month: %s", parts[1])
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %s", parts[2])
	}

	// 民國年轉西元（三位數年份 < 1000 視為民國）
	if year < 1000 {
		year += 1911
	}

	if year < 1900 || year > 2200 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("date out of range: %s", s)
	}

	loc := time.FixedZone("CST", 8*3600)
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), nil
}
