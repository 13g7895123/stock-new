package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

const TWSEListedURL = "https://openapi.twse.com.tw/v1/opendata/t187ap03_L"

var regularStockPattern = regexp.MustCompile(`^[1-9]\d{3}$`)

type TWSeStock struct {
	Symbol string `json:"公司代號"`
	Name   string `json:"公司簡稱"`
}

func FetchListedStocks() ([]TWSeStock, error) {
	req, err := http.NewRequest(http.MethodGet, TWSEListedURL, nil)
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

	var all []TWSeStock
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	var result []TWSeStock
	for _, s := range all {
		if regularStockPattern.MatchString(s.Symbol) {
			result = append(result, s)
		}
	}

	return result, nil
}
