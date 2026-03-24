package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const TPEXOtcURL = "https://www.tpex.org.tw/openapi/v1/tpex_mainboard_quotes"

type tpexQuote struct {
	Symbol   string `json:"SecuritiesCompanyCode"`
	Name     string `json:"CompanyName"`
	Industry string `json:"IndustryName"`
}

// FetchOtcStocks 從 TPEX 抓取上櫃股票清單，套用相同的四碼非零開頭過濾
func FetchOtcStocks() ([]TWSeStock, error) {
	req, err := http.NewRequest(http.MethodGet, TPEXOtcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var all []tpexQuote
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	var result []TWSeStock
	for _, s := range all {
		if regularStockPattern.MatchString(s.Symbol) {
			result = append(result, TWSeStock{Symbol: s.Symbol, Name: s.Name, Industry: s.Industry})
		}
	}

	return result, nil
}
