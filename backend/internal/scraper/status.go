package scraper

import (
	"encoding/json"
	"fmt"
	htmlstd "html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"

	"github.com/PuerkitoBio/goquery"
)

const (
	TWSEDispositionURL          = "https://www.twse.com.tw/announcement/punish?response=html"
	TWSEAttentionURL            = "https://www.twse.com.tw/zh/announcement/notice.html"
	TWSEAttentionAccumURL       = "https://www.twse.com.tw/zh/announcement/notetrans.html"
	TWSEDayTradePreviewURL      = "https://www.twse.com.tw/zh/trading/day-trading/twtbau.html"
	TWSEDayTradeHistoryURL      = "https://www.twse.com.tw/zh/trading/day-trading/twtbau-history.html"
	TPEXDispositionURL          = "https://www.tpex.org.tw/openapi/v1/tpex_disposal_information"
	TPEXAttentionURL            = "https://www.tpex.org.tw/openapi/v1/tpex_trading_warning_information"
	statusHTTPTimeout           = 20 * time.Second
	stockStatusSourceMarketTWSE = "TWSE"
	stockStatusSourceMarketTPEX = "TPEX"
)

var htmlBreakPattern = regexp.MustCompile(`(?i)<br\s*/?>`)

type StockStatusRecord struct {
	Symbol     string
	Name       string
	Market     string
	Type       string
	SourceDate time.Time
	StartDate  time.Time
	EndDate    time.Time
	Reason     string
	Measure    string
	Detail     string
	RawPeriod  string
	SourceURL  string
	FetchedAt  time.Time
}

type tpexDispositionRecord struct {
	Date                  string `json:"Date"`
	SecuritiesCompanyCode string `json:"SecuritiesCompanyCode"`
	CompanyName           string `json:"CompanyName"`
	DispositionPeriod     string `json:"DispositionPeriod"`
	DispositionReasons    string `json:"DispositionReasons"`
	DisposalCondition     string `json:"DisposalCondition"`
}

type twseTablePayload struct {
	Stat   string   `json:"stat"`
	Date   string   `json:"date"`
	Title  string   `json:"title"`
	Fields []string `json:"fields"`
	Data   [][]any  `json:"data"`
}

type twseStatusRow map[string]string

func (r twseStatusRow) value(candidates ...string) string {
	for _, candidate := range candidates {
		if v := strings.TrimSpace(r[normalizeTWSEFieldKey(candidate)]); v != "" {
			return v
		}
	}
	return ""
}

type tpexAttentionRecord struct {
	Date                  string `json:"Date"`
	SecuritiesCompanyCode string `json:"SecuritiesCompanyCode"`
	CompanyName           string `json:"CompanyName"`
	TradingInformation    string `json:"TradingInformation"`
}

func FetchOfficialStockStatuses(referenceDate time.Time) ([]StockStatusRecord, error) {
	fetchedAt := time.Now().In(time.FixedZone("CST", 8*3600))
	if referenceDate.IsZero() {
		referenceDate = fetchedAt
	}
	referenceDate = dateOnly(referenceDate)

	fetchers := []func(time.Time, time.Time) ([]StockStatusRecord, error){
		fetchTWSEDispositionStatuses,
		fetchTWSEAttentionStatuses,
		fetchTWSEAttentionAccumStatuses,
		fetchTWSEDayTradeRestrictedStatuses,
		fetchTPEXDispositionStatuses,
		fetchTPEXAttentionStatuses,
	}

	var out []StockStatusRecord
	for _, fetcher := range fetchers {
		records, err := fetcher(referenceDate, fetchedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, records...)
	}
	return out, nil
}

func fetchTWSEDispositionStatuses(_ time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	rows, err := fetchTWSEHTMLTableRows(TWSEDispositionURL)
	if err != nil {
		return nil, fmt.Errorf("fetch TWSE disposition: %w", err)
	}

	out := make([]StockStatusRecord, 0, len(rows))
	for _, row := range rows {
		code := row.value("證券代號")
		if !regularStockPattern.MatchString(code) {
			continue
		}
		rawPeriod := row.value("處置起迄時間", "處置期間")
		startDate, endDate, err := parseROCPeriod(rawPeriod)
		if err != nil {
			continue
		}
		sourceDate, err := parseROCDateFlexible(row.value("公布日期"))
		if err != nil {
			sourceDate = startDate
		}
		reason := row.value("處置條件", "處置原因")
		measure := row.value("處置措施")
		detail := joinNonEmpty("\n", row.value("處置內容"), row.value("備註"))
		base := StockStatusRecord{
			Symbol:     strings.TrimSpace(code),
			Name:       row.value("證券名稱"),
			Market:     stockStatusSourceMarketTWSE,
			SourceDate: dateOnly(sourceDate),
			StartDate:  startDate,
			EndDate:    endDate,
			Reason:     reason,
			Measure:    measure,
			Detail:     detail,
			RawPeriod:  rawPeriod,
			SourceURL:  TWSEDispositionURL,
			FetchedAt:  fetchedAt,
		}
		base.Type = models.StockStatusDisposition
		out = append(out, base)
		if containsDayTradeRestriction(measure + detail) {
			dayTrade := base
			dayTrade.Type = models.StockStatusDayTradeRestricted
			out = append(out, dayTrade)
		}
	}
	return out, nil
}

func fetchTWSEAttentionStatuses(referenceDate time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	payload, err := fetchTWSEJSONTableFromPage(TWSEAttentionURL)
	if err != nil {
		return nil, fmt.Errorf("fetch TWSE attention: %w", err)
	}

	rows := twseRowsFromPayload(payload)
	out := make([]StockStatusRecord, 0, len(rows))
	for _, row := range rows {
		code := row.value("證券代號")
		if !regularStockPattern.MatchString(code) {
			continue
		}
		sourceDate, err := parseROCDateFlexible(row.value("日期"))
		if err != nil {
			sourceDate = latestROCDateInText(payload.Title, referenceDate)
		}
		sourceDate = dateOnly(sourceDate)
		out = append(out, StockStatusRecord{
			Symbol:     strings.TrimSpace(code),
			Name:       row.value("證券名稱"),
			Market:     stockStatusSourceMarketTWSE,
			Type:       models.StockStatusAttention,
			SourceDate: sourceDate,
			StartDate:  sourceDate,
			EndDate:    sourceDate,
			Reason:     row.value("注意交易資訊", "注 意交易資訊"),
			Measure:    row.value("累計次數"),
			SourceURL:  TWSEAttentionURL,
			FetchedAt:  fetchedAt,
		})
	}
	return out, nil
}

func fetchTWSEAttentionAccumStatuses(referenceDate time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	payload, err := fetchTWSEJSONTableFromPage(TWSEAttentionAccumURL)
	if err != nil {
		return nil, fmt.Errorf("fetch TWSE attention accumulation: %w", err)
	}

	rows := twseRowsFromPayload(payload)
	out := make([]StockStatusRecord, 0, len(rows))
	for _, row := range rows {
		code := row.value("證券代號")
		if !regularStockPattern.MatchString(code) {
			continue
		}
		reason := row.value("近期達本公司公布注意交易資訊標準之情形", "近期達本公司「公布注意交易資訊」標準之情形")
		sourceDate := latestROCDateInText(reason, latestROCDateInText(payload.Title, referenceDate))
		out = append(out, StockStatusRecord{
			Symbol:     strings.TrimSpace(code),
			Name:       row.value("證券名稱"),
			Market:     stockStatusSourceMarketTWSE,
			Type:       models.StockStatusAttention,
			SourceDate: sourceDate,
			StartDate:  sourceDate,
			EndDate:    sourceDate,
			Reason:     reason,
			SourceURL:  TWSEAttentionAccumURL,
			FetchedAt:  fetchedAt,
		})
	}
	return out, nil
}

func fetchTWSEDayTradeRestrictedStatuses(referenceDate time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	pageURLs := []string{TWSEDayTradePreviewURL, TWSEDayTradeHistoryURL}
	out := make([]StockStatusRecord, 0)
	for _, pageURL := range pageURLs {
		records, err := fetchTWSEDayTradeRestrictedPage(pageURL, referenceDate, fetchedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, records...)
	}
	return out, nil
}

func fetchTWSEDayTradeRestrictedPage(pageURL string, referenceDate time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	payload, err := fetchTWSEJSONTableFromPage(pageURL)
	if err != nil {
		return nil, fmt.Errorf("fetch TWSE day trade restricted: %w", err)
	}

	rows := twseRowsFromPayload(payload)
	out := make([]StockStatusRecord, 0, len(rows))
	for _, row := range rows {
		code := row.value("證券代號", "股票代號")
		if !regularStockPattern.MatchString(code) {
			continue
		}
		startRaw := row.value("停止先賣後買開始日")
		endRaw := row.value("停止先賣後買結束日")
		startDate, err := parseROCDateFlexible(startRaw)
		if err != nil {
			continue
		}
		endDate, err := parseROCDateFlexible(endRaw)
		if err != nil {
			endDate = startDate
		}
		startDate = dateOnly(startDate)
		endDate = dateOnly(endDate)
		sourceDate := latestROCDateInText(payload.Title, referenceDate)
		out = append(out, StockStatusRecord{
			Symbol:     strings.TrimSpace(code),
			Name:       row.value("證券名稱", "股票名稱"),
			Market:     stockStatusSourceMarketTWSE,
			Type:       models.StockStatusDayTradeRestricted,
			SourceDate: sourceDate,
			StartDate:  startDate,
			EndDate:    endDate,
			Reason:     row.value("原因"),
			Measure:    "暫停先賣後買當日沖銷交易",
			Detail:     payload.Title,
			RawPeriod:  joinNonEmpty(" ~ ", startRaw, endRaw),
			SourceURL:  pageURL,
			FetchedAt:  fetchedAt,
		})
	}
	return out, nil
}

func fetchTPEXDispositionStatuses(_ time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	var records []tpexDispositionRecord
	if err := fetchStatusJSON(TPEXDispositionURL, &records); err != nil {
		return nil, fmt.Errorf("fetch TPEX disposition: %w", err)
	}

	out := make([]StockStatusRecord, 0, len(records))
	for _, record := range records {
		if !regularStockPattern.MatchString(record.SecuritiesCompanyCode) {
			continue
		}
		startDate, endDate, err := parseROCPeriod(record.DispositionPeriod)
		if err != nil {
			continue
		}
		sourceDate, err := parseROCDateFlexible(record.Date)
		if err != nil {
			sourceDate = startDate
		}
		base := StockStatusRecord{
			Symbol:     strings.TrimSpace(record.SecuritiesCompanyCode),
			Name:       strings.TrimSpace(record.CompanyName),
			Market:     stockStatusSourceMarketTPEX,
			SourceDate: dateOnly(sourceDate),
			StartDate:  startDate,
			EndDate:    endDate,
			Reason:     strings.TrimSpace(record.DispositionReasons),
			Measure:    strings.TrimSpace(record.DisposalCondition),
			Detail:     strings.TrimSpace(record.DisposalCondition),
			RawPeriod:  strings.TrimSpace(record.DispositionPeriod),
			SourceURL:  TPEXDispositionURL,
			FetchedAt:  fetchedAt,
		}
		base.Type = models.StockStatusDisposition
		out = append(out, base)
		if containsDayTradeRestriction(record.DisposalCondition) {
			dayTrade := base
			dayTrade.Type = models.StockStatusDayTradeRestricted
			out = append(out, dayTrade)
		}
	}
	return out, nil
}

func fetchTPEXAttentionStatuses(referenceDate time.Time, fetchedAt time.Time) ([]StockStatusRecord, error) {
	var records []tpexAttentionRecord
	if err := fetchStatusJSON(TPEXAttentionURL, &records); err != nil {
		return nil, fmt.Errorf("fetch TPEX attention: %w", err)
	}

	out := make([]StockStatusRecord, 0, len(records))
	for _, record := range records {
		if !regularStockPattern.MatchString(record.SecuritiesCompanyCode) {
			continue
		}
		sourceDate, err := parseROCDateFlexible(record.Date)
		if err != nil {
			sourceDate = referenceDate
		}
		sourceDate = dateOnly(sourceDate)
		out = append(out, StockStatusRecord{
			Symbol:     strings.TrimSpace(record.SecuritiesCompanyCode),
			Name:       strings.TrimSpace(record.CompanyName),
			Market:     stockStatusSourceMarketTPEX,
			Type:       models.StockStatusAttention,
			SourceDate: sourceDate,
			StartDate:  sourceDate,
			EndDate:    sourceDate,
			Reason:     strings.TrimSpace(record.TradingInformation),
			SourceURL:  TPEXAttentionURL,
			FetchedAt:  fetchedAt,
		})
	}
	return out, nil
}

func fetchStatusJSON(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: statusHTTPTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func fetchStatusDocument(rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := (&http.Client{Timeout: statusHTTPTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

func fetchTWSEHTMLTableRows(pageURL string) ([]twseStatusRow, error) {
	doc, err := fetchStatusDocument(pageURL)
	if err != nil {
		return nil, err
	}
	table := doc.Find("table").First()
	if table.Length() == 0 {
		return nil, fmt.Errorf("table not found")
	}

	headers := selectionCells(table.Find("thead tr").Last(), "th,td")
	if len(headers) == 0 {
		headers = selectionCells(table.Find("tr").First(), "th,td")
	}
	if len(headers) == 0 {
		return nil, fmt.Errorf("table headers not found")
	}

	rows := make([]twseStatusRow, 0)
	table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		if tr.Find("td").Length() == 0 {
			return
		}
		cells := selectionCells(tr, "td")
		if len(cells) == 0 {
			return
		}
		row := twseRowFromCells(headers, cells)
		if len(row) > 0 {
			rows = append(rows, row)
		}
	})
	return rows, nil
}

func fetchTWSEJSONTableFromPage(pageURL string) (twseTablePayload, error) {
	endpoint, err := resolveTWSEDataEndpoint(pageURL)
	if err != nil {
		return twseTablePayload{}, err
	}
	var payload twseTablePayload
	if err := fetchStatusJSON(endpoint, &payload); err != nil {
		return payload, err
	}
	if payload.Stat != "" && payload.Stat != "OK" {
		return payload, fmt.Errorf("TWSE response: %s", payload.Stat)
	}
	return payload, nil
}

func resolveTWSEDataEndpoint(pageURL string) (string, error) {
	doc, err := fetchStatusDocument(pageURL)
	if err != nil {
		return "", err
	}
	apiPath := ""
	doc.Find("[data-api]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		apiPath = strings.TrimSpace(attrOrEmpty(s, "data-api"))
		return apiPath == ""
	})
	if apiPath == "" {
		return "", fmt.Errorf("data-api not found")
	}
	return buildTWSEJSONEndpoint(pageURL, apiPath)
}

func buildTWSEJSONEndpoint(pageURL, apiPath string) (string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	var endpoint *url.URL
	if strings.HasPrefix(apiPath, "http://") || strings.HasPrefix(apiPath, "https://") {
		endpoint, err = url.Parse(apiPath)
	} else {
		origin := base.Scheme + "://" + base.Host
		path := strings.TrimSpace(apiPath)
		if strings.HasPrefix(path, "/rwd/") {
			endpoint, err = url.Parse(origin + path)
		} else {
			endpoint, err = url.Parse(origin + "/rwd/zh/" + strings.TrimPrefix(path, "/"))
		}
	}
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	if query.Get("response") == "" {
		query.Set("response", "json")
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func attrOrEmpty(s *goquery.Selection, name string) string {
	v, ok := s.Attr(name)
	if !ok {
		return ""
	}
	return v
}

func selectionCells(row *goquery.Selection, selector string) []string {
	cells := make([]string, 0)
	row.Find(selector).Each(func(_ int, cell *goquery.Selection) {
		raw, err := cell.Html()
		if err != nil {
			raw = cell.Text()
		}
		cells = append(cells, cleanCellText(raw))
	})
	return cells
}

func twseRowsFromPayload(payload twseTablePayload) []twseStatusRow {
	rows := make([]twseStatusRow, 0, len(payload.Data))
	for _, cells := range payload.Data {
		cellTexts := make([]string, 0, len(cells))
		for _, cell := range cells {
			cellTexts = append(cellTexts, cleanCellText(anyToString(cell)))
		}
		row := twseRowFromCells(payload.Fields, cellTexts)
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func twseRowFromCells(headers, cells []string) twseStatusRow {
	row := twseStatusRow{}
	for i, header := range headers {
		if i >= len(cells) {
			break
		}
		key := normalizeTWSEFieldKey(header)
		if key == "" {
			continue
		}
		row[key] = strings.TrimSpace(cells[i])
	}
	return row
}

func normalizeTWSEFieldKey(s string) string {
	s = cleanCellText(s)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "　", "", "\u00a0", "", "\"", "")
	return replacer.Replace(s)
}

func anyToString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func cleanCellText(raw string) string {
	normalized := htmlBreakPattern.ReplaceAllString(raw, "\n")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + normalized + "</div>"))
	if err == nil {
		normalized = doc.Find("div").First().Text()
	}
	normalized = htmlstd.UnescapeString(normalized)
	normalized = strings.ReplaceAll(normalized, "\u00a0", " ")
	normalized = strings.ReplaceAll(normalized, "　", " ")

	lines := strings.Split(normalized, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, separator)
}

func parseROCPeriod(raw string) (time.Time, time.Time, error) {
	parts := splitPeriod(raw)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid ROC period: %s", raw)
	}
	startDate, err := parseROCDateFlexible(parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDate, err := parseROCDateFlexible(parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return dateOnly(startDate), dateOnly(endDate), nil
}

func splitPeriod(raw string) []string {
	replacer := strings.NewReplacer("～", "~", "－", "~", "—", "~", "至", "~")
	normalized := replacer.Replace(strings.TrimSpace(raw))
	parts := strings.Split(normalized, "~")
	if len(parts) != 2 {
		return nil
	}
	return []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
}

func parseROCDateFlexible(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty ROC date")
	}

	if strings.Contains(s, "年") {
		re := regexp.MustCompile(`(\d{2,3})年\s*(\d{1,2})月\s*(\d{1,2})日`)
		match := re.FindStringSubmatch(s)
		if len(match) == 4 {
			return parseROCDateParts(match[1], match[2], match[3])
		}
	}

	if strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		if len(parts) == 3 {
			return parseROCDateParts(parts[0], parts[1], parts[2])
		}
	}

	digits := regexp.MustCompile(`\D`).ReplaceAllString(s, "")
	if len(digits) == 8 {
		return parseROCDateParts(digits[:4], digits[4:6], digits[6:8])
	}
	if len(digits) == 7 {
		return parseROCDateParts(digits[:3], digits[3:5], digits[5:7])
	}
	return time.Time{}, fmt.Errorf("invalid ROC date: %s", raw)
}

func parseROCDateParts(yearRaw, monthRaw, dayRaw string) (time.Time, error) {
	year, err := strconv.Atoi(strings.TrimSpace(yearRaw))
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(strings.TrimSpace(monthRaw))
	if err != nil {
		return time.Time{}, err
	}
	day, err := strconv.Atoi(strings.TrimSpace(dayRaw))
	if err != nil {
		return time.Time{}, err
	}
	if year < 1911 {
		year += 1911
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

func latestROCDateInText(text string, fallback time.Time) time.Time {
	var latest time.Time
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d{2,4})年\s*(\d{1,2})月\s*(\d{1,2})日`),
		regexp.MustCompile(`(\d{2,4})/(\d{1,2})/(\d{1,2})`),
	}
	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) != 4 {
				continue
			}
			parsed, err := parseROCDateParts(match[1], match[2], match[3])
			if err == nil && (latest.IsZero() || parsed.After(latest)) {
				latest = parsed
			}
		}
	}
	if latest.IsZero() {
		return dateOnly(fallback)
	}
	return dateOnly(latest)
}

func containsDayTradeRestriction(text string) bool {
	return strings.Contains(text, "當日沖") || strings.Contains(text, "日沖銷")
}

func dateOnly(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
