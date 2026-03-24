---
name: stock-chips-pyramid
description: 'Taiwan stock holder distribution scraper system. Use when: developing features, debugging scraper, extending API, modifying data pipeline, adding charts, working on job queue, understanding system architecture, or maintaining the stock-chips-pyramid project. Covers: data sources (norway.twsthr.info, TWSE API, TPEx API), Go backend, Python Playwright scraper, React frontend, PostgreSQL schema, Redis queue/pubsub, WebSocket real-time updates.'
argument-hint: 'feature area (scraper | backend | frontend | db | queue)'
---

# Stock Chips Pyramid — 軟體工程知識庫

台灣股票持股分佈爬蟲系統。透過 Playwright 爬取持股金字塔資料，以 Go 後端提供 REST API 與 WebSocket，React 前端視覺化呈現。

---

## 系統架構

```
┌──────────────┐   LPUSH    ┌─────────────────┐   BRPOP   ┌──────────────────────┐
│  Go Backend  │ ─────────► │   Redis Queue   │ ────────► │  Python Scraper      │
│  (Gin + WS)  │            │  scrape_queue   │           │  (Playwright async)  │
└──────┬───────┘            └─────────────────┘           └──────────┬───────────┘
       │ REST/WS                                                      │ asyncpg save
       │                   ┌─────────────────┐                       │
       │◄──SUBSCRIBE────── │  Redis Pub/Sub  │ ◄──PUBLISH────────────┘
       │   scrape_events   └─────────────────┘
       │
       │ pgxpool
       ▼
┌──────────────┐       ┌──────────────────────────────────────────────────────┐
│  PostgreSQL  │       │  React Frontend (Vite + Tailwind + Recharts)         │
│  stockchips  │       │  - StocksPage: 股票列表 + 搜尋                        │
└──────────────┘       │  - StockDetailPage: 持股分佈長條圖 + 歷史快照          │
                       │  - JobsPage: 爬取任務管理 + WebSocket 即時進度         │
                       └──────────────────────────────────────────────────────┘
```

**技術棧**

| 層級 | 技術 |
|------|------|
| Backend | Go 1.22, Gin, gorilla/websocket, pgx/v5, go-redis/v9 |
| Scraper | Python 3.12, Playwright (async), asyncpg, BeautifulSoup4, tenacity |
| Frontend | React 18, TypeScript, Vite, Tailwind CSS, Recharts, axios |
| Database | PostgreSQL 16 + pg_trgm extension |
| Queue / Pub-Sub | Redis 7 (List + Pub/Sub) |
| 容器 | Docker Compose |

---

## 資料來源

### 1. 股票清單同步（後端主動呼叫）

| 市場 | URL | 呼叫方式 |
|------|-----|----------|
| 上市 TWSE | `https://openapi.twse.com.tw/v1/exchangeReport/STOCK_DAY_ALL` | HTTP GET JSON |
| 上櫃 TPEx | `https://www.tpex.org.tw/openapi/v1/tpex_mainboard_daily_close_quotes` | HTTP GET JSON |

- 由 `POST /api/sync-stocks` 觸發
- 實作位置：[backend/sync.go](../../../backend/sync.go)  
- 邏輯：先將所有 `is_active = FALSE`，再批次 UPSERT，最終只有 API 回傳的股票為 `is_active = TRUE`
- 股票代碼驗證：4 位純數字，不以 `0` 開頭（`isSupportedStockCode`）

### 2. 持股分佈資料（爬蟲）

- **來源網站**：`https://norway.twsthr.info/StockHolders.aspx?stock={股票代碼}`
- **爬取方式**：Playwright 無頭瀏覽器（模擬真實瀏覽器，附 `zh-TW` Accept-Language header）
- **等待策略**：`wait_until="networkidle"` + 額外 800ms（等待 JS 渲染）
- **重試機制**：最多 3 次，指數退避（2s → 4s → 8s）
- **解析器**：[scraper/scraper/parser.py](../../../scraper/scraper/parser.py)
  - 日期格式：民國年 `YYY/MM/DD` 自動轉西元
  - 持股區間識別：`_is_distribution_label()` 過濾合計列
  - 數值清洗：去除逗號、百分比符號、處理 `-` / `N/A` 為 None

---

## 資料庫 Schema

詳見 [migrations/001_init.sql](../../../migrations/001_init.sql)

```
stocks
├── code (PK, VARCHAR 10)       -- 股票代碼 e.g. "2330"
├── name                        -- 公司名稱
├── market                      -- "TWSE" | "TPEx"
├── industry                    -- 產業別
├── is_active                   -- 是否仍在市 (sync 時更新)
└── last_scraped_at             -- 最後爬取時間

scrape_jobs
├── id (PK, BIGSERIAL)
├── status                      -- pending | running | completed | failed | cancelled
├── total_count / success_count / fail_count
└── started_at / completed_at

holder_snapshots
├── id (PK, BIGSERIAL)
├── stock_code (FK → stocks)
├── job_id (FK → scrape_jobs)
├── data_date (DATE)            -- 頁面上的資料日期（民國年轉換後）
└── UNIQUE (stock_code, data_date)  -- 同日期不重複存入

holder_distributions
├── id (PK, BIGSERIAL)
├── snapshot_id (FK → holder_snapshots, CASCADE DELETE)
├── tier_rank                   -- 表格列順序 (1-based)
├── range_label                 -- 持股區間 e.g. "1 ~ 999 股"
├── holder_count / holder_pct   -- 持有人數 / 佔比 %
├── share_count / share_pct     -- 持有股數 / 佔比 %
├── cum_holder_pct              -- 累積人數佔比
└── cum_share_pct               -- 累積股數佔比
```

索引：`pg_trgm` GIN index on `stocks.name`（支援模糊搜尋）

---

## API 端點

完整文件見 [references/api-endpoints.md](./references/api-endpoints.md)

| Method | Path | 說明 |
|--------|------|------|
| GET | `/api/stocks` | 列出股票（支援 `q`, `market`, `page`, `per_page`） |
| GET | `/api/stocks/:code` | 單一股票詳情 |
| GET | `/api/stocks/:code/snapshots` | 歷史快照列表（`limit` 最大 100） |
| GET | `/api/stocks/:code/snapshots/latest` | 最新快照（含分佈明細） |
| GET | `/api/snapshots/:id` | 特定快照詳情 |
| GET | `/api/jobs` | 任務列表（最近 50 筆） |
| GET | `/api/jobs/:id` | 單一任務詳情 |
| POST | `/api/jobs` | 建立爬取任務（body: `{"stock_codes": ["2330",...]}` 或空 body 爬全部） |
| POST | `/api/jobs/:id/cancel` | 取消任務 |
| POST | `/api/sync-stocks` | 同步 TWSE + TPEx 股票清單 |
| GET | `/ws` | WebSocket 連線（即時進度廣播） |
| GET | `/health` | 健康檢查 |

---

## Queue / Pub-Sub 流程

```
建立任務 (POST /api/jobs)
  └─► DB: INSERT scrape_jobs (status='running')
  └─► Redis LPUSH scrape_queue: [{job_id, stock_code}, ...]
  └─► Redis PUBLISH scrape_events: {type:"job_started", ...}

Scraper Worker (BRPOP scrape_queue)
  └─► Playwright 爬取 norway.twsthr.info
  └─► asyncpg 儲存至 holder_snapshots + holder_distributions
  └─► Redis PUBLISH scrape_events: {type:"progress", status:"success"|"failed", ...}
  └─► 全部完成後: PUBLISH {type:"job_completed", ...}

Backend (SUBSCRIBE scrape_events)
  └─► WebSocket broadcast 給所有前端連線
```

- Queue key：`scrape_queue`（Redis List，LPUSH 入隊，BRPOP 消費）  
- Pub-Sub channel：`scrape_events`  
- Worker 並發度：`Config.CONCURRENCY`（預設 5，由 `asyncio.Semaphore` 控制）

---

## WebSocket 事件格式

前端 `useWebSocket` hook 自動重連（間隔 3 秒）。

```typescript
interface WSEvent {
  type: 'progress' | 'job_started' | 'job_completed' | 'job_failed'
  job_id: number
  stock_code?: string        // 'progress' 時有值
  status?: 'success' | 'failed'  // 'progress' 時有值
  success_count: number
  fail_count: number
  total_count: number
  message?: string
}
```

---

## 前端頁面

| 頁面 | 路由 | 功能 |
|------|------|------|
| StocksPage | `/` | 股票列表、分頁、市場篩選、模糊搜尋 |
| StockDetailPage | `/stocks/:code` | 股票資訊、最新持股金字塔長條圖（Recharts Bar）、歷史快照選擇器、單支股票爬取按鈕 |
| JobsPage | `/jobs` | 任務管理、建立全量爬取任務、取消任務、WebSocket 即時進度 log |

---

## 環境變數

| 服務 | 變數 | 預設值 |
|------|------|--------|
| Backend | `DATABASE_URL` | `postgresql://chips:chips_secret@localhost:5432/stockchips` |
| Backend | `REDIS_URL` | `redis://localhost:6379` |
| Backend | `PORT` | `8080` |
| Backend | `GIN_MODE` | `debug` |
| Scraper | `DATABASE_URL` | 同上 |
| Scraper | `REDIS_URL` | 同上 |
| Scraper | `CONCURRENCY` | `5` |
| Scraper | `REQUEST_DELAY` | `2.0` (秒) |
| Scraper | `HEADLESS` | `true` |

---

## 常見開發任務

### 新增持股分佈分析欄位

1. `migrations/` 新增 SQL — ALTER TABLE `holder_distributions` ADD COLUMN  
2. `scraper/scraper/models.py` — `HolderRow` dataclass 加欄位  
3. `scraper/scraper/parser.py` — `_parse_row()` 解析新欄位  
4. `scraper/scraper/db.py` — INSERT 語句加欄位  
5. `backend/models.go` — `HolderDistribution` struct 加 JSON 欄位  
6. `backend/db.go` — SELECT query 加欄位  
7. `frontend/src/api.ts` — interface 加欄位  
8. `frontend/src/pages/StockDetailPage.tsx` — 圖表加新資料系列  

### 新增 API 端點

1. `backend/handlers.go` — 實作 handler func  
2. `backend/db.go` — 實作 DB query（使用 `pgxpool.Pool`）  
3. `backend/main.go` — 在 `api` group 注冊路由  
4. `frontend/src/api.ts` — 新增 axios 呼叫函式  

### 調整爬蟲並發 / 速率

- 修改 `CONCURRENCY` env var（Docker Compose `scraper` service）  
- 修改 `REQUEST_DELAY` 增加請求間隔  
- `Config.NAV_TIMEOUT`（ms）控制 Playwright 導航逾時  
- `Config.MAX_RETRIES` 控制最大重試次數  

### 本地啟動

```bash
make up        # docker compose up -d --build
make logs      # docker compose logs -f
make down      # docker compose down
make psql      # 進入 PostgreSQL 互動介面
```

詳細 targets 見 [Makefile](../../../Makefile)。

---

## 除錯指引

| 問題 | 排查方向 |
|------|----------|
| 爬蟲爬不到資料 | 檢查 `norway.twsthr.info` 頁面結構是否改版；查看 `parser.py` `_is_distribution_label` 與 `_is_data_row` 條件 |
| 快照已存在衝突 | `holder_snapshots` 有 UNIQUE(stock_code, data_date)；相同日期重複爬取會跳過（`ON CONFLICT DO NOTHING`）|
| Redis queue 積壓 | 檢查 scraper container 是否正常運行；`redis-cli LLEN scrape_queue` 查看積壓數量 |
| WebSocket 斷線 | 前端 `useWebSocket` 會自動重連；後端 hub broadcast 錯誤不影響其他連線 |
| 股票列表無資料 | 呼叫 `POST /api/sync-stocks` 從 TWSE/TPEx 同步股票清單 |
