# API 端點完整規格

## 股票相關

### GET /api/stocks
列出股票清單，支援搜尋與分頁。

**Query Parameters**
| 參數 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `q` | string | - | 模糊搜尋（股票代碼或名稱，透過 pg_trgm LIKE） |
| `market` | string | - | `TWSE` 或 `TPEx` |
| `page` | int | 1 | 頁碼（1-based） |
| `per_page` | int | 50 | 每頁筆數（最大 200） |

**Response 200**
```json
{
  "data": [
    {
      "code": "2330",
      "name": "台積電",
      "market": "TWSE",
      "industry": "半導體業",
      "is_active": true,
      "last_scraped_at": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1800,
  "page": 1,
  "per_page": 50
}
```

---

### GET /api/stocks/:code
取得單一股票詳情。

**Response 200** — `Stock` 物件  
**Response 404** — `{"error": "not found"}`

---

### GET /api/stocks/:code/snapshots
取得某支股票的歷史快照列表（不含分佈明細）。

**Query Parameters**
| 參數 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `limit` | int | 10 | 最多回傳筆數（最大 100） |

**Response 200**
```json
{
  "data": [
    {
      "id": 42,
      "stock_code": "2330",
      "data_date": "2024-01-10",
      "scraped_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

---

### GET /api/stocks/:code/snapshots/latest
取得最新快照（**含** `distributions` 陣列）。

**Response 200** — `HolderSnapshot` 物件（含 distributions）  
**Response 404** — `{"error": "no snapshot found"}`

---

### GET /api/snapshots/:id
依 ID 取得特定快照（含分佈明細）。

**Response 200** — `HolderSnapshot` 物件（含 distributions）  
**Response 400** — `{"error": "invalid id"}`  
**Response 404** — `{"error": "not found"}`

---

## 爬取任務相關

### GET /api/jobs
取得最近 50 筆爬取任務（依建立時間倒序）。

**Response 200**
```json
{
  "data": [
    {
      "id": 1,
      "status": "completed",
      "total_count": 1800,
      "success_count": 1795,
      "fail_count": 5,
      "started_at": "2024-01-15T10:00:00Z",
      "completed_at": "2024-01-15T12:30:00Z",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

---

### GET /api/jobs/:id
取得單一任務詳情。

---

### POST /api/jobs
建立新的爬取任務，並立即推入 Redis queue。

**Request Body（選填）**
```json
{ "stock_codes": ["2330", "2317"] }
```
- 若 `stock_codes` 為空（或不傳 body）→ 爬取**所有** `is_active=TRUE` 的股票

**Response 201** — `ScrapeJob` 物件  
**Response 400** — `{"error": "no stocks found; sync stock list first"}` （DB 中無股票）

---

### POST /api/jobs/:id/cancel
取消指定任務（僅限 `pending` 或 `running` 狀態）。

**Response 200** — `{"ok": true}`  
**Response 404** — 任務不存在

---

## 系統管理

### POST /api/sync-stocks
從 TWSE + TPEx Open API 同步最新股票清單至 DB。

**Response 200**
```json
{ "upserted": 1842 }
```

---

### GET /health
健康檢查。

**Response 200**
```json
{ "status": "ok" }
```

---

## WebSocket

### GET /ws (Upgrade)
升級為 WebSocket 連線。後端會廣播 `scrape_events` Redis channel 的所有訊息。

**Event Types**

| type | 觸發時機 | 額外欄位 |
|------|----------|----------|
| `job_started` | 任務建立後 | `total_count` |
| `progress` | 每支股票完成爬取 | `stock_code`, `status` ("success"/"failed"), `success_count`, `fail_count`, `total_count` |
| `job_completed` | 全部股票處理完畢 | `success_count`, `fail_count`, `total_count` |
| `job_failed` | 任務整體失敗 | `message` |
