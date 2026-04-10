package scraper

// institutional.go — 三大法人每日買賣超資料爬取
//
// 資料來源：
//   上市(TWSE): https://www.twse.com.tw/rwd/zh/fund/T86?response=json&date=YYYYMMDD&selectType=ALL
//   上櫃(TPEX): https://www.tpex.org.tw/openapi/v1/tpex_mainboard_institution_trading_statistics
//
// TWSE T86 回應格式（JSON）：
//   fields: ["證券代號","證券名稱","外陸資買進股數(不含外資自營商)","外陸資賣出股數(不含外資自營商)",
//            "外陸資買賣超股數(不含外資自營商)","外資自營商買進股數","外資自營商賣出股數",
//            "外資自營商買賣超股數","投信買進股數","投信賣出股數","投信買賣超股數",
//            "自營商買賣超股數","自營商買進股數(自行買賣)","自營商賣出股數(自行買賣)",
//            "自營商買賣超股數(自行買賣)","自營商買進股數(避險)","自營商賣出股數(避險)",
//            "自營商買賣超股數(避險)","三大法人買賣超股數"]
//   data:   [["1101","台泥","18,907,101",...],...]
//
// TPEX 回應格式（JSON array）：
//   每筆物件欄位（英文 key）依 OpenAPI

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// InstitutionalRecord 單一股票單日的三大法人買賣超
type InstitutionalRecord struct {
	Symbol      string
	Date        time.Time
	Market      string // TWSE | TPEX
	ForeignBuy  int64
	ForeignSell int64
	ForeignNet  int64
	TrustBuy    int64
	TrustSell   int64
	TrustNet    int64
	DealerNet   int64
	TotalNet    int64
}

// ─── TWSE T86 ────────────────────────────────────────────────────────────────

type twseT86Response struct {
	Stat   string     `json:"stat"`
	Date   string     `json:"date"`
	Fields []string   `json:"fields"`
	Data   [][]string `json:"data"`
}

// FetchTWSEInstitutional 抓取上市三大法人當日買賣超（或指定日期）
func FetchTWSEInstitutional(date time.Time) ([]InstitutionalRecord, error) {
	dateStr := date.Format("20060102")
	url := fmt.Sprintf(
		"https://www.twse.com.tw/rwd/zh/fund/T86?response=json&date=%s&selectType=ALL",
		dateStr,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch TWSE T86 failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TWSE T86 unexpected status: %d", resp.StatusCode)
	}

	var result twseT86Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode TWSE T86 failed: %w", err)
	}
	if result.Stat != "OK" {
		// 非交易日或資料不存在
		return nil, nil
	}

	records := make([]InstitutionalRecord, 0, len(result.Data))
	for _, row := range result.Data {
		if len(row) < 19 {
			continue
		}
		symbol := strings.TrimSpace(row[0])
		if !regularStockPattern.MatchString(symbol) {
			continue
		}

		// row 欄位索引（依 T86 fields 順序）：
		// 0=代號, 1=名稱,
		// 2=外資買進, 3=外資賣出, 4=外資買賣超（不含外資自營商）
		// 5=外資自營商買進, 6=外資自營商賣出, 7=外資自營商買賣超
		// 8=投信買進, 9=投信賣出, 10=投信買賣超
		// 11=自營商合計買賣超, 12=自行買進, 13=自行賣出, 14=自行買賣超, 15=避險買進, 16=避險賣出, 17=避險買賣超
		// 18=三大法人合計買賣超
		rec := InstitutionalRecord{
			Symbol:      symbol,
			Date:        date,
			Market:      "TWSE",
			ForeignBuy:  parseCommaSep(row[2]),
			ForeignSell: parseCommaSep(row[3]),
			ForeignNet:  parseCommaSep(row[4]),
			TrustBuy:    parseCommaSep(row[8]),
			TrustSell:   parseCommaSep(row[9]),
			TrustNet:    parseCommaSep(row[10]),
			DealerNet:   parseCommaSep(row[11]),
			TotalNet:    parseCommaSep(row[18]),
		}
		records = append(records, rec)
	}
	return records, nil
}

// ─── TPEX ─────────────────────────────────────────────────────────────────────

type tpexInstitutionalItem struct {
	Date              string `json:"date"`
	SecuritiesCode    string `json:"SecuritiesCompanyCode"`
	ForeignBuyShares  string `json:"ForeignInvestorBuyShares"`
	ForeignSellShares string `json:"ForeignInvestorSellShares"`
	ForeignNetShares  string `json:"ForeignInvestorNetBuySellShares"`
	TrustBuyShares    string `json:"InvestmentTrustBuyShares"`
	TrustSellShares   string `json:"InvestmentTrustSellShares"`
	TrustNetShares    string `json:"InvestmentTrustNetBuySellShares"`
	DealerNetShares   string `json:"DealerNetBuySellShares"`
	TotalNetShares    string `json:"TotalInstitutionalNetBuySellShares"`
}

// FetchTPEXInstitutional 抓取上櫃三大法人當日買賣超
func FetchTPEXInstitutional(date time.Time) ([]InstitutionalRecord, error) {
	dateStr := fmt.Sprintf("%d/%02d/%02d",
		date.Year()-1911, date.Month(), date.Day()) // ROC format: 115/04/10

	url := fmt.Sprintf(
		"https://www.tpex.org.tw/web/stock/3insti/daily_trade/3itrade_hedge_result.php?l=zh-tw&t=D&se=EW&d=%s&s=0,asc,0",
		dateStr,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json, text/javascript")
	req.Header.Set("Referer", "https://www.tpex.org.tw/")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch TPEX institutional failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TPEX institutional unexpected status: %d", resp.StatusCode)
	}

	// TPEX 回傳格式：{"iTotalRecords":N,"aaData":[["代號","名稱","外資買","外資賣","外資超",...],...]}}
	var raw struct {
		TotalRecords int        `json:"iTotalRecords"`
		Data         [][]string `json:"aaData"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode TPEX institutional failed: %w", err)
	}

	records := make([]InstitutionalRecord, 0, len(raw.Data))
	for _, row := range raw.Data {
		// TPEX aaData 欄位依序：
		// 0=代號, 1=名稱,
		// 2=外資買進, 3=外資賣出, 4=外資買賣超,
		// 5=投信買進, 6=投信賣出, 7=投信買賣超,
		// 8=自營商買賣超(自行), 9=自營商買賣超(避險), 10=自營商合計買賣超,
		// 11=三大法人合計買賣超
		if len(row) < 12 {
			continue
		}
		symbol := strings.TrimSpace(row[0])
		if !regularStockPattern.MatchString(symbol) {
			continue
		}

		rec := InstitutionalRecord{
			Symbol:      symbol,
			Date:        date,
			Market:      "TPEX",
			ForeignBuy:  parseCommaSep(row[2]),
			ForeignSell: parseCommaSep(row[3]),
			ForeignNet:  parseCommaSep(row[4]),
			TrustBuy:    parseCommaSep(row[5]),
			TrustSell:   parseCommaSep(row[6]),
			TrustNet:    parseCommaSep(row[7]),
			DealerNet:   parseCommaSep(row[10]),
			TotalNet:    parseCommaSep(row[11]),
		}
		records = append(records, rec)
	}
	return records, nil
}

// ─── 工具函式 ────────────────────────────────────────────────────────────────

// parseCommaSep 解析帶千分位逗號的整數字串，如 "18,907,101" → 18907101
// 支援負數（帶前綴 "-"）
func parseCommaSep(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	if s == "" || s == "--" || s == "---" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
