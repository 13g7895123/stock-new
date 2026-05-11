# 全市場前兩個交易日價量 API

> 日期：2026-05-10  
> 方法：GET  
> 範圍：全市場上市 / 上櫃股票最近兩個有日 K 資料的交易日收盤價、成交量，以及本地已同步的處置股 / 注意股 / 限當沖股標記

---

## 端點

```http
GET /api/prices/previous-trading-days
```

### Query 參數

| 參數 | 必填 | 說明 | 範例 |
|------|------|------|------|
| `as_of` | 否 | 查詢截止日期，格式為 `YYYY-MM-DD`；未提供時使用台北時區今天 | `2026-05-10` |
| `market` | 否 | 市場篩選；未提供時同時回傳上市與上櫃 | `TWSE` / `TPEX` |

---

## 查詢邏輯

API 會從 `stocks` 取上市 / 上櫃股票，對每一檔股票從 `daily_prices` 查詢 `date <= as_of` 且日期由新到舊的最近 2 筆，再回傳股票名稱、市場別、收盤價與成交量。

同時會從 `stock_statuses` 查詢 `start_date <= as_of <= end_date` 的有效標記，回傳：

- `is_disposition`：是否為處置股
- `is_attention`：是否為注意股
- `is_day_trade_restricted`：是否為限當沖股
- `statuses`：有效標記明細，包含來源日期、起訖日期、原因與處置措施

因為交易日資料只會在有開盤的日期寫入，週末、國定假日或休市日會自然被略過。若某檔股票資料不足 2 筆，會回傳該檔實際可取得筆數；若查無任何資料，回傳空陣列。

此端點是全市場查詢，不需要前端依股票代號逐檔發 request。

處置股與限當沖股會依處置期間判斷有效性，例如處置期間到 `2026-05-08`，用 `as_of=2026-05-09` 查詢時不會再出現標記。注意股官方資料沒有固定結束日期，本系統以同步到的公告日期作為單日有效標記；若要查歷史注意股，需確保該日曾執行狀態同步。

## 狀態同步

股票狀態採 DB 同步方式，不在查詢 API 時即時打交易所。

上市狀態來源會從 `docs/twse_urls_zh_tw.md` 記錄的 TWSE 官方頁面進入：

- 處置股：`https://www.twse.com.tw/announcement/punish?response=html`
- 注意股：`https://www.twse.com.tw/zh/announcement/notice.html`
- 注意累計異常：`https://www.twse.com.tw/zh/announcement/notetrans.html`
- 限當沖股：`https://www.twse.com.tw/zh/trading/day-trading/twtbau.html`、`https://www.twse.com.tw/zh/trading/day-trading/twtbau-history.html`

其中注意股與限當沖頁面是前端渲染頁，後端會先抓官方頁面並解析 `data-api`，再讀取對應 `response=json` 資料。上櫃狀態維持使用 TPEx 官方 OpenAPI 作為來源。

```http
POST /api/stock-statuses/sync
GET /api/stock-statuses/status
GET /api/stock-statuses?as_of=2026-05-08&type=disposition&market=TWSE
GET /api/stock-statuses?symbol=2330
```

`GET /api/stock-statuses` 可使用下列 Query 參數：

| 參數 | 必填 | 說明 | 範例 |
|------|------|------|------|
| `as_of` | 否 | 查詢有效日期，格式 `YYYY-MM-DD`；未提供時使用台北時區今天 | `2026-05-10` |
| `type` | 否 | 狀態類型 | `disposition` / `attention` / `day_trade_restricted` |
| `market` | 否 | 市場篩選 | `TWSE` / `TPEX` |
| `symbol` | 否 | 股票代號篩選 | `2330` |

排程任務 ID：`stock_status`。可透過 `/api/schedules` 設定每日同步。

---

## 回應範例

### 200 OK

```json
{
  "as_of": "2026-05-10",
  "count": 2,
  "data": [
    {
      "symbol": "2330",
      "name": "台積電",
      "market": "TWSE",
      "is_disposition": false,
      "is_attention": false,
      "is_day_trade_restricted": false,
      "statuses": [],
      "data": [
        {
          "date": "2026-05-08",
          "close": 875,
          "volume": 38214567
        },
        {
          "date": "2026-05-07",
          "close": 868,
          "volume": 29401822
        }
      ]
    },
    {
      "symbol": "6488",
      "name": "環球晶",
      "market": "TPEX",
      "is_disposition": true,
      "is_attention": false,
      "is_day_trade_restricted": true,
      "statuses": [
        {
          "type": "disposition",
          "label": "處置股",
          "source_date": "2026-05-08",
          "start_date": "2026-05-08",
          "end_date": "2026-05-12",
          "reason": "因達處置標準",
          "measure": "處置期間限制當日沖銷交易"
        },
        {
          "type": "day_trade_restricted",
          "label": "限當沖股",
          "source_date": "2026-05-08",
          "start_date": "2026-05-08",
          "end_date": "2026-05-12",
          "reason": "因達處置標準",
          "measure": "處置期間限制當日沖銷交易"
        }
      ],
      "data": [
        {
          "date": "2026-05-08",
          "close": 472.5,
          "volume": 5112000
        },
        {
          "date": "2026-05-07",
          "close": 469,
          "volume": 4389000
        }
      ]
    }
  ]
}
```

### 400 Bad Request

`as_of` 格式錯誤時回傳：

```json
{
  "error": "as_of must be YYYY-MM-DD"
}
```

或 `market` 不是 `TWSE` / `TPEX` 時回傳：

```json
{
  "error": "market must be TWSE or TPEX"
}
```

### 無資料

查無任何符合條件的日 K 價量資料時，仍回傳 `200 OK`：

```json
{
  "as_of": "2026-05-10",
  "count": 0,
  "data": []
}
```

---

## 使用範例

```bash
curl "http://localhost:8080/api/prices/previous-trading-days"
```

指定截止日期：

```bash
curl "http://localhost:8080/api/prices/previous-trading-days?as_of=2026-05-10"
```

只查上市：

```bash
curl "http://localhost:8080/api/prices/previous-trading-days?market=TWSE"
```

只查上櫃：

```bash
curl "http://localhost:8080/api/prices/previous-trading-days?market=TPEX"
```