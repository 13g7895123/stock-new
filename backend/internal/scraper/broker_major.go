package scraper

// broker_major.go — 主力買賣超（券商分點進出）爬取
//
// 資料來源：MoneyDJ 券商代理站台 → /z/zc/zco/zco_{symbol}.djhtm
//
// URL 格式：
//   單日（今日）：{base}/z/zc/zco/zco_{symbol}.djhtm
//   N 日累計：   {base}/z/zc/zco/zco_{symbol}_{N}.djhtm  （N=5/10/20/40/60）
//
// 頁面為 Big5 編碼 HTML，需先 decode 再交給 goquery 解析。
// 每筆資料列含 10 個 TD：左 5 欄為買超券商，右 5 欄為賣超券商。
//
// 可用 domain（本機壓測，5/6 通過；sjmain.esunsec.com.tw 已失效）：
//   http://fubon-ebrokerdj.fbs.com.tw
//   http://jdata.yuanta.com.tw
//   http://newjust.masterlink.com.tw
//   http://kgieworld.moneydj.com
//   http://justdata.moneydj.com

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"

	"stock-backend/internal/models"
)

// MajorBrokerBaseURLs — 主力進出可用券商 domain 清單，依序 failover。
// 排除已知失效的 sjmain.esunsec.com.tw。
var MajorBrokerBaseURLs = []string{
	"http://justdata.moneydj.com",
	"http://kgieworld.moneydj.com",
	"http://jdata.yuanta.com.tw",
	"http://fubon-ebrokerdj.fbs.com.tw",
	"http://newjust.masterlink.com.tw",
}

// MajorFetchResult 爬取主力進出的結果與診斷資訊
type MajorFetchResult struct {
	Source   string    // 成功的 base URL
	URL      string    // 完整請求 URL
	DataDate time.Time // 頁面資料日期
	Records  []models.MajorBrokerRecord
	Tried    []string // 嘗試過但失敗的 URL
}

// BuildMajorURL 建構主力進出請求 URL。
// days <= 1 時取今日單日資料；否則取近 N 日累計。
func BuildMajorURL(base, symbol string, days int) string {
	base = strings.TrimRight(base, "/")
	if days <= 1 {
		return fmt.Sprintf("%s/z/zc/zco/zco_%s.djhtm", base, symbol)
	}
	return fmt.Sprintf("%s/z/zc/zco/zco_%s_%d.djhtm", base, symbol, days)
}

// FetchMajorBrokers 爬取主力進出，自動 failover 各 domain。
func FetchMajorBrokers(symbol string, days int) (*MajorFetchResult, error) {
	return FetchMajorBrokersWith(symbol, days, MajorBrokerBaseURLs)
}

// FetchMajorBrokersWith 允許指定 baseURLs（方便單元測試）。
func FetchMajorBrokersWith(symbol string, days int, baseURLs []string) (*MajorFetchResult, error) {
	result := &MajorFetchResult{}
	for _, base := range baseURLs {
		url := BuildMajorURL(base, symbol, days)
		dataDate, records, err := fetchAndParseMajor(symbol, url, days)
		if err == nil {
			result.Source = base
			result.URL = url
			result.DataDate = dataDate
			result.Records = records
			return result, nil
		}
		result.Tried = append(result.Tried, fmt.Sprintf("%s: %v", url, err))
	}
	return result, fmt.Errorf("all brokers failed or returned no data")
}

// ─── 內部實作 ─────────────────────────────────────────────────

var majorDateRe = regexp.MustCompile(`最後更新日[：:]\s*(\d{4}/\d{1,2}/\d{1,2})`)

func fetchAndParseMajor(symbol, url string, days int) (time.Time, []models.MajorBrokerRecord, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9")
	req.Header.Set("Referer", "http://www.moneydj.com/")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Big5 → UTF-8 解碼，直接流式傳給 goquery
	decoder := traditionalchinese.Big5.NewDecoder()
	reader := transform.NewReader(resp.Body, decoder)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("parse HTML: %w", err)
	}

	// 從 body 全文中提取資料日期
	bodyText := doc.Find("body").Text()
	dataDate := time.Time{}
	taipei := time.FixedZone("Asia/Taipei", 8*3600)
	if m := majorDateRe.FindStringSubmatch(bodyText); len(m) > 1 {
		if t, err := time.ParseInLocation("2006/1/2", m[1], taipei); err == nil {
			dataDate = t
		}
	}
	if dataDate.IsZero() {
		// 後備：以今日本地時間 00:00 作為資料日期
		now := time.Now().In(taipei)
		dataDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, taipei)
	}

	records := parseMajorTable(doc, symbol, dataDate, days)
	// 允許「無資料」（股票當日無主力進出），不視為錯誤
	return dataDate, records, nil
}

// parseMajorTable 解析 #oMainTable 中的買超 / 賣超資料列。
//
// 結構：每個資料列包含 10 個 TD：
//
//	[0] 買超券商(t4t1)  [1-3] 買進/賣出/買超(t3n1)  [4] 比重(t3n1)
//	[5] 賣超券商(t4t1)  [6-8] 買進/賣出/賣超(t3n1)  [9] 比重(t3n1)
func parseMajorTable(doc *goquery.Document, symbol string, dataDate time.Time, days int) []models.MajorBrokerRecord {
	table := doc.Find("#oMainTable")
	if table.Length() == 0 {
		return nil
	}

	now := time.Now()
	var records []models.MajorBrokerRecord
	rowIdx := 0

	table.Find("tr, TR").Each(func(_ int, row *goquery.Selection) {
		cells := make([]string, 0, 10)
		row.Find("td, TD").Each(func(_ int, td *goquery.Selection) {
			class, _ := td.Attr("class")
			if strings.Contains(class, "t4t1") || strings.Contains(class, "t3n1") {
				cells = append(cells, strings.TrimSpace(td.Text()))
			}
		})
		if len(cells) != 10 {
			return
		}
		rowIdx++
		rank := rowIdx

		// 買超側
		buyBroker := cells[0]
		buyBuyVol := parseMajorInt(cells[1])
		buySellVol := parseMajorInt(cells[2])
		buyNetVol := parseMajorInt(cells[3])
		buyPct := parseMajorPercent(cells[4])

		// 賣超側（net_vol 存負值方便前端判斷方向）
		sellBroker := cells[5]
		sellBuyVol := parseMajorInt(cells[6])
		sellSellVol := parseMajorInt(cells[7])
		sellNetVol := -parseMajorInt(cells[8])
		sellPct := parseMajorPercent(cells[9])

		if buyBroker != "" && buyBroker != "\u00a0" {
			records = append(records, models.MajorBrokerRecord{
				Symbol:     symbol,
				DataDate:   dataDate,
				Days:       days,
				Side:       "buy",
				Rank:       rank,
				BrokerName: buyBroker,
				BuyVol:     buyBuyVol,
				SellVol:    buySellVol,
				NetVol:     buyNetVol,
				Pct:        buyPct,
				ScrapedAt:  now,
			})
		}
		if sellBroker != "" && sellBroker != "\u00a0" {
			records = append(records, models.MajorBrokerRecord{
				Symbol:     symbol,
				DataDate:   dataDate,
				Days:       days,
				Side:       "sell",
				Rank:       rank,
				BrokerName: sellBroker,
				BuyVol:     sellBuyVol,
				SellVol:    sellSellVol,
				NetVol:     sellNetVol,
				Pct:        sellPct,
				ScrapedAt:  now,
			})
		}
	})

	return records
}

func parseMajorInt(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseMajorPercent(s string) float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
