# TWSE 可爬網址清單

本文整理臺灣證券交易所（TWSE）與台股「處置股、注意股、限當沖股」相關的官方查詢頁面，方便作為爬蟲開發、資料蒐集與監控用途的起點。[cite:12][cite:13][cite:16]

## 處置股

TWSE 提供「公布處置有價證券資訊」頁面，可查詢最新處置股公告與處置期間等資訊。[cite:12] 另外，也可使用較直接的查詢路徑取得 HTML 回應，較適合程式端自動化抓取。[cite:20]

| 類別 | 網址 | 用途說明 |
|---|---|---|
| 最新處置股公告 | [https://www.twse.com.tw/zh/announcement/punish.html](https://www.twse.com.tw/zh/announcement/punish.html) | 查詢當前處置股清單、處置條件、起訖日期與撮合方式。[cite:12] |
| HTML 查詢入口 | [https://www.twse.com.tw/announcement/punish?response=html](https://www.twse.com.tw/announcement/punish?response=html) | 可作為程式抓取入口，適合解析 HTML 表格內容。[cite:20] |

## 注意股

TWSE 提供「公布注意有價證券」頁面，作為每日注意股公告的官方來源。[cite:13] 另有「公布注意累計次數異常資訊」頁面，可追蹤累計次數異常的相關資訊。[cite:21]

| 類別 | 網址 | 用途說明 |
|---|---|---|
| 最新注意股公告 | [https://www.twse.com.tw/zh/announcement/notice.html](https://www.twse.com.tw/zh/announcement/notice.html) | 查詢每日注意股名單與注意原因。[cite:13] |
| 注意累計次數異常 | [https://www.twse.com.tw/zh/announcement/notetrans.html](https://www.twse.com.tw/zh/announcement/notetrans.html) | 查詢注意次數累計異常資訊，適合搭配處置條件規則分析。[cite:21] |

## 限當沖股

TWSE 提供「暫停先賣後買當日沖銷交易標的預告表」，可查詢次一交易日受限當沖的標的。[cite:16] 此外，也提供每月統計與歷史查詢頁，方便做回測、事件研究或歷史資料蒐集。[cite:25][cite:28]

| 類別 | 網址 | 用途說明 |
|---|---|---|
| 每日限當沖預告 | [https://www.twse.com.tw/zh/trading/day-trading/twtbau.html](https://www.twse.com.tw/zh/trading/day-trading/twtbau.html) | 查詢暫停先賣後買當日沖銷交易標的預告表。[cite:16] |
| 每月當沖標的及統計 | [https://www.twse.com.tw/zh/trading/day-trading/twtb4u-month.html](https://www.twse.com.tw/zh/trading/day-trading/twtb4u-month.html) | 查詢每月當沖標的與統計資料。[cite:25] |
| 限當沖歷史查詢 | [https://www.twse.com.tw/zh/trading/day-trading/twtbau-history.html](https://www.twse.com.tw/zh/trading/day-trading/twtbau-history.html) | 查詢歷史暫停當沖資料，適合建立歷史事件資料庫。[cite:28] |

## 爬取建議

這些 TWSE 頁面多半屬於表格型公告頁，適合先使用 `pandas.read_html()` 嘗試解析；若表格結構不穩，再改用 `requests` 搭配 `BeautifulSoup` 抽取欄位。[cite:12][cite:13][cite:16] 開發時建議保留公告日期、股票代號、股票名稱、公告原因、處置起日、處置迄日與交易限制類型等欄位，便於後續比對與監控。[cite:12][cite:16]
