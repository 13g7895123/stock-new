package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

const TWSEListedURL = "https://openapi.twse.com.tw/v1/opendata/t187ap03_L"

var regularStockPattern = regexp.MustCompile(`^[1-9]\d{3}$`)

var twseIndustryMap = map[string]string{
	"01": "水泥工業",
	"02": "食品工業",
	"03": "塑膠工業",
	"04": "紡織纖維",
	"05": "電機機械",
	"06": "電器電纜",
	"08": "化學工業",
	"09": "生技醫療業",
	"10": "玻璃陶瓷",
	"11": "造紙工業",
	"12": "鋼鐵工業",
	"13": "橡膠工業",
	"14": "汽車工業",
	"15": "電子工業",
	"16": "建材營造業",
	"17": "航運業",
	"18": "觀光餐旅",
	"19": "金融業",
	"20": "貿易百貨業",
	"21": "綜合",
	"22": "其他",
	"24": "油電燃氣業",
	"25": "半導體業",
	"26": "電腦及週邊設備業",
	"27": "光電業",
	"28": "通信網路業",
	"29": "電子零組件業",
	"30": "電子通路業",
	"31": "資訊服務業",
	"32": "其他電子業",
	"33": "文化創意業",
	"34": "農業科技業",
	"35": "電子商務業",
	"36": "綠能環保",
	"37": "數位雲端",
	"38": "運動休閒",
	"39": "居家生活",
}

// ResolveIndustry 將 TWSE 產業別欄位轉為中文名稱（若非數字代碼則原樣返回）
func ResolveIndustry(code string) string {
	if name, ok := twseIndustryMap[code]; ok {
		return name
	}
	return code
}

type TWSeStock struct {
	Symbol   string `json:"公司代號"`
	Name     string `json:"公司簡稱"`
	Industry string `json:"產業別"`
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
			s.Industry = ResolveIndustry(s.Industry)
			result = append(result, s)
		}
	}

	return result, nil
}
