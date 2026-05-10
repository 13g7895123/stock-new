# 前兩個交易日價量 API

> 日期：2026-05-10  
> 方法：GET  
> 範圍：單一股票最近兩個有日 K 資料的交易日收盤價與成交量

---

## 端點

```http
GET /api/stocks/:symbol/prices/previous-trading-days
```

### Path 參數

| 參數 | 必填 | 說明 | 範例 |
|------|------|------|------|
| `symbol` | 是 | 股票代號 | `2330` |

### Query 參數

| 參數 | 必填 | 說明 | 範例 |
|------|------|------|------|
| `as_of` | 否 | 查詢截止日期，格式為 `YYYY-MM-DD`；未提供時使用台北時區今天 | `2026-05-10` |

---

## 查詢邏輯

API 會從 `daily_prices` 依股票代號查詢 `date <= as_of` 的資料，依日期由新到舊排序，取最近 2 筆。

因為交易日資料只會在有開盤的日期寫入，週末、國定假日或休市日會自然被略過。若資料不足 2 筆，會回傳實際可取得筆數；若完全沒有價量資料，回傳 `404`。

---

## 回應範例

### 200 OK

```json
{
  "symbol": "2330",
  "count": 2,
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
}
```

### 400 Bad Request

`as_of` 格式錯誤時回傳：

```json
{
  "error": "as_of must be YYYY-MM-DD"
}
```

### 404 Not Found

指定股票沒有日 K 價量資料時回傳：

```json
{
  "error": "no price data"
}
```

---

## 使用範例

```bash
curl "http://localhost:8080/api/stocks/2330/prices/previous-trading-days"
```

指定截止日期：

```bash
curl "http://localhost:8080/api/stocks/2330/prices/previous-trading-days?as_of=2026-05-10"
```