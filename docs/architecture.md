# 台股分析系統 — 架構設計文件

> 版本：v1.0  
> 日期：2026-03-22  
> 範圍：歷史資料蒐集、籌碼分析、價量分析（不含即時報價）

---

## 目錄

1. [系統概覽](#1-系統概覽)
2. [資料來源清單](#2-資料來源清單)
3. [資料庫設計](#3-資料庫設計)
4. [後端 API 設計](#4-後端-api-設計)
5. [資料蒐集排程](#5-資料蒐集排程)
6. [目錄結構](#6-目錄結構)
7. [Docker 部署架構](#7-docker-部署架構)
8. [導入優先順序](#8-導入優先順序)

---

## 1. 系統概覽

```
┌─────────────────────────────────────────────────────────┐
│                      資料蒐集層                          │
│  TWSE OpenAPI ──┐                                       │
│  TPEX OpenAPI ──┼──→ Go Scraper ──→ Job Queue          │
│  公開資訊觀測站 ──┘        （cron 排程）                  │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                       儲存層                             │
│  TimescaleDB (PostgreSQL + 時序擴充套件)                  │
│  ├── 參考資料：stocks, dividends                         │
│  ├── 時序資料：daily_prices, institutional_trades        │
│  │              margin_trading, securities_lending       │
│  └── 財報資料：financials (JSONB)                        │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                    應用服務層                             │
│  Go REST API (Gin)                                       │
│  ├── 股票基本資料 CRUD                                   │
│  ├── 價量查詢（日K / 週K / 月K）                         │
│  ├── 籌碼查詢（三大法人 / 融資券）                        │
│  └── 技術指標計算（MA / RSI / MACD，API 層即時運算）      │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                      展示層                              │
│  Nuxt 4 (Vue 3)                                         │
│  ├── 股票列表與搜尋                                      │
│  ├── 個股頁面（K 線圖 / 籌碼圖）                         │
│  └── 同步進度（SSE 即時推送）                            │
└─────────────────────────────────────────────────────────┘
```

---

## 2. 資料來源清單

### 2.1 股票基本資料（每日同步）

| 市場 | API | 說明 |
|------|-----|------|
| 上市 | `https://openapi.twse.com.tw/v1/opendata/t187ap03_L` | TWSE 上市公司基本資料，包含代號、簡稱、產業別、上市日期 |
| 上櫃 | `https://www.tpex.org.tw/openapi/v1/tpex_mainboard_quotes` | TPEX 上櫃股票報價（含公司名稱） |

**過濾規則：** 正規表達式 `^[1-9]\d{3}$`（非零開頭、四碼純數字，排除 ETF、DR、指數）

---

### 2.2 日K 價量資料（每日收盤後）

| 市場 | API | 格式 |
|------|-----|------|
| 上市個股 | `https://www.twse.com.tw/exchangeReport/STOCK_DAY?stockNo={symbol}&date={YYYYMMDD}` | JSON |
| 上櫃個股 | `https://www.tpex.org.tw/openapi/v1/tpex_mainboard_daily_close_quotes` | JSON |

**欄位說明：**

```
日期、開盤、最高、最低、收盤、成交量（股）、成交金額（元）、成交筆數
```

**注意：** 上市 API 以月份為單位回傳，每次請求一個月的資料。

---

### 2.3 三大法人買賣超（每日收盤後）

| 市場 | API |
|------|-----|
| 上市 | `https://www.twse.com.tw/fund/T86?response=json&date={YYYYMMDD}&selectType=ALLBUT0999` |
| 上櫃 | `https://www.tpex.org.tw/openapi/v1/tpex_3insti_daily_close_quotes` |

**包含：** 外資（含外資自營）、投信、自營商 的買進、賣出、淨買超張數。

---

### 2.4 融資融券餘額（每日收盤後）

| 市場 | API |
|------|-----|
| 上市 | `https://www.twse.com.tw/exchangeReport/MI_MARGN?response=json&date={YYYYMMDD}&selectType=ALL` |
| 上櫃 | `https://www.tpex.org.tw/openapi/v1/tpex_margin_balance_daily` |

**包含：** 融資餘額/增減、融券餘額/增減、資券相抵張數。

---

### 2.5 借券異動（每日收盤後）

| 市場 | API |
|------|-----|
| 上市 | `https://www.twse.com.tw/exchangeReport/TWT93U?response=json&date={YYYYMMDD}` |

**包含：** 借券賣出量、借券餘額、還券量。

---

### 2.6 除權息資訊（不定期）

| 來源 | API |
|------|-----|
| 上市 | `https://www.twse.com.tw/exchangeReport/TWT48U?response=json&strDate={YYYYMMDD}&endDate={YYYYMMDD}` |

**用途：** 計算還原收盤價（Adjusted Close），讓 K 線不因除權息出現跳空缺口。

---

### 2.7 季度財報（每季）

| 來源 | 方式 |
|------|------|
| 公開資訊觀測站 | HTML 爬取或 XBRL API（`https://mops.twse.com.tw`） |

**包含：** 營收、毛利、營業利益、淨利、EPS、ROE、負債比率。

> ⚠️ 公開資訊觀測站無標準化 JSON API，需針對財報種類（合併/個別）分別處理，難度較高，建議最後導入。

---

## 3. 資料庫設計

### 3.1 為什麼選 TimescaleDB

TimescaleDB 是 PostgreSQL 的開源擴充套件，在原本 PostgreSQL 基礎上增加：

| 功能 | 說明 |
|------|------|
| **自動時間分區（Hypertable）** | 依時間切分成 chunk，查詢自動跳過不相關的分區 |
| **壓縮** | 歷史資料壓縮率高達 90%，節省磁碟空間 |
| **連續聚合（Continuous Aggregate）** | 預計算週K/月K，無需即時 GROUP BY |
| **完整 PostgreSQL 相容** | GORM、SQL 指令完全不需修改 |

**docker-compose 修改只需一行：**
```yaml
image: timescale/timescaledb:latest-pg16  # 取代 postgres:16-alpine
```

---

### 3.2 完整 Schema

#### 啟用擴充套件

```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
```

---

#### 股票主檔（擴充現有 stocks 表）

```sql
ALTER TABLE stocks
  ADD COLUMN market       VARCHAR(10)  NOT NULL DEFAULT 'listed',
  -- listed（上市）/ otc（上櫃）
  ADD COLUMN industry     VARCHAR(50),
  ADD COLUMN listed_date  DATE,
  ADD COLUMN is_active    BOOLEAN      NOT NULL DEFAULT TRUE;

CREATE INDEX ON stocks (market);
CREATE INDEX ON stocks (industry);
```

---

#### 日K 價量（時序主表）

```sql
CREATE TABLE daily_prices (
  time          TIMESTAMPTZ   NOT NULL,
  symbol        VARCHAR(10)   NOT NULL,
  open          NUMERIC(10,2) NOT NULL,
  high          NUMERIC(10,2) NOT NULL,
  low           NUMERIC(10,2) NOT NULL,
  close         NUMERIC(10,2) NOT NULL,
  adj_close     NUMERIC(10,2),           -- 還原收盤價
  volume        BIGINT        NOT NULL,  -- 成交量（股）
  tx_value      BIGINT,                  -- 成交金額（元）
  tx_count      INT                      -- 成交筆數
);

-- 轉為 hypertable（以月為單位分區）
SELECT create_hypertable('daily_prices', 'time');
SELECT set_chunk_time_interval('daily_prices', INTERVAL '1 month');

-- 唯一索引（防重複寫入）
CREATE UNIQUE INDEX ON daily_prices (symbol, time DESC);

-- 壓縮設定（超過 2 年自動壓縮）
ALTER TABLE daily_prices SET (
  timescaledb.compress,
  timescaledb.compress_orderby    = 'time DESC',
  timescaledb.compress_segmentby  = 'symbol'
);
SELECT add_compression_policy('daily_prices', INTERVAL '2 years');
```

---

#### 預計算週K / 月K（連續聚合）

```sql
-- 週K（自動增量更新，不需手動維護）
CREATE MATERIALIZED VIEW weekly_prices
WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 week', time) AS week,
  symbol,
  first(open,  time) AS open,
  max(high)          AS high,
  min(low)           AS low,
  last(close,  time) AS close,
  sum(volume)        AS volume,
  sum(tx_value)      AS tx_value
FROM daily_prices
GROUP BY week, symbol
WITH NO DATA;

SELECT add_continuous_aggregate_policy('weekly_prices',
  start_offset => INTERVAL '1 month',
  end_offset   => INTERVAL '1 day',
  schedule_interval => INTERVAL '1 day'
);

-- 月K（同上）
CREATE MATERIALIZED VIEW monthly_prices
WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 month', time) AS month,
  symbol,
  first(open,  time) AS open,
  max(high)          AS high,
  min(low)           AS low,
  last(close,  time) AS close,
  sum(volume)        AS volume,
  sum(tx_value)      AS tx_value
FROM daily_prices
GROUP BY month, symbol
WITH NO DATA;
```

---

#### 三大法人買賣超

```sql
CREATE TABLE institutional_trades (
  time         TIMESTAMPTZ NOT NULL,
  symbol       VARCHAR(10) NOT NULL,

  -- 外資（含外資自營）
  foreign_buy   BIGINT DEFAULT 0,   -- 買進張數
  foreign_sell  BIGINT DEFAULT 0,   -- 賣出張數
  foreign_net   BIGINT DEFAULT 0,   -- 淨買超張數

  -- 投信
  trust_buy     BIGINT DEFAULT 0,
  trust_sell    BIGINT DEFAULT 0,
  trust_net     BIGINT DEFAULT 0,

  -- 自營商（含避險）
  dealer_buy    BIGINT DEFAULT 0,
  dealer_sell   BIGINT DEFAULT 0,
  dealer_net    BIGINT DEFAULT 0,

  -- 合計
  total_net     BIGINT GENERATED ALWAYS AS
                (foreign_net + trust_net + dealer_net) STORED
);

SELECT create_hypertable('institutional_trades', 'time');
CREATE UNIQUE INDEX ON institutional_trades (symbol, time DESC);
```

---

#### 融資融券餘額

```sql
CREATE TABLE margin_trading (
  time              TIMESTAMPTZ NOT NULL,
  symbol            VARCHAR(10) NOT NULL,

  -- 融資
  margin_balance    BIGINT DEFAULT 0,   -- 融資餘額（張）
  margin_buy        BIGINT DEFAULT 0,   -- 融資買進
  margin_sell       BIGINT DEFAULT 0,   -- 融資賣出
  margin_repay      BIGINT DEFAULT 0,   -- 現償

  -- 融券
  short_balance     BIGINT DEFAULT 0,   -- 融券餘額（張）
  short_buy         BIGINT DEFAULT 0,   -- 券買進（還券）
  short_sell        BIGINT DEFAULT 0,   -- 券賣出
  short_cover       BIGINT DEFAULT 0,   -- 現券

  -- 其他
  offset_shares     BIGINT DEFAULT 0,   -- 資券相抵張數
  margin_ratio      NUMERIC(6,4),       -- 融資使用率（%）
  short_ratio       NUMERIC(6,4)        -- 融券使用率（%）
);

SELECT create_hypertable('margin_trading', 'time');
CREATE UNIQUE INDEX ON margin_trading (symbol, time DESC);
```

---

#### 借券

```sql
CREATE TABLE securities_lending (
  time              TIMESTAMPTZ NOT NULL,
  symbol            VARCHAR(10) NOT NULL,
  lending_balance   BIGINT DEFAULT 0,   -- 借券餘額（股）
  lending_sell      BIGINT DEFAULT 0,   -- 借券賣出
  return_shares     BIGINT DEFAULT 0    -- 還券
);

SELECT create_hypertable('securities_lending', 'time');
CREATE UNIQUE INDEX ON securities_lending (symbol, time DESC);
```

---

#### 除權息事件

```sql
CREATE TABLE dividends (
  symbol           VARCHAR(10)   NOT NULL,
  ex_date          DATE          NOT NULL,  -- 除權息日
  cash_dividend    NUMERIC(8, 4) NOT NULL DEFAULT 0,   -- 現金股利（元）
  stock_dividend   NUMERIC(8, 4) NOT NULL DEFAULT 0,   -- 股票股利（股/千股）
  reference_price  NUMERIC(10,2),                       -- 除權息參考價
  filled_date      DATE,                                 -- 填息日（事後回填）
  PRIMARY KEY (symbol, ex_date)
);
```

---

#### 季度財報

```sql
CREATE TABLE financials (
  symbol           VARCHAR(10) NOT NULL,
  year             SMALLINT    NOT NULL,
  quarter          SMALLINT    NOT NULL CHECK (quarter BETWEEN 1 AND 4),

  -- 損益表常用指標
  revenue          BIGINT,               -- 營業收入（千元）
  gross_profit     BIGINT,               -- 毛利
  operating_income BIGINT,               -- 營業利益
  net_income       BIGINT,               -- 本期淨利
  eps              NUMERIC(8, 4),        -- 每股盈餘

  -- 財務比率
  roe              NUMERIC(8, 4),        -- 股東權益報酬率（%）
  roa              NUMERIC(8, 4),        -- 資產報酬率（%）
  gross_margin     NUMERIC(8, 4),        -- 毛利率（%）
  operating_margin NUMERIC(8, 4),        -- 營益率（%）
  debt_ratio       NUMERIC(8, 4),        -- 負債比率（%）

  -- 完整原始數據（保留所有欄位，方便未來擴充）
  raw              JSONB,

  PRIMARY KEY (symbol, year, quarter)
);

CREATE INDEX ON financials (symbol, year DESC, quarter DESC);
CREATE INDEX ON financials USING gin (raw);  -- 支援 JSONB 任意欄位查詢
```

---

### 3.3 索引策略總覽

| 查詢類型 | 索引 |
|----------|------|
| 單支股票時間範圍查詢 | `(symbol, time DESC)` 唯一索引 |
| 某日全市場掃描（選股） | `(time DESC)` 由 hypertable 分區優化 |
| 產業別篩選 | `stocks(industry)` |
| 財報任意欄位查詢 | `financials` GIN index on `raw` |

---

## 4. 後端 API 設計

### 4.1 路由規劃

```
GET  /health

# 股票基本資料
GET  /api/stocks                     ?market=listed|otc&industry=&page=&size=
GET  /api/stocks/:symbol             取得單支股票資訊

# 價量資料
GET  /api/stocks/:symbol/prices      ?from=2024-01-01&to=2024-12-31&interval=daily|weekly|monthly
GET  /api/stocks/:symbol/prices/latest   最新一筆

# 籌碼資料
GET  /api/stocks/:symbol/institution ?from=&to=   三大法人
GET  /api/stocks/:symbol/margin      ?from=&to=   融資融券
GET  /api/stocks/:symbol/lending     ?from=&to=   借券

# 財報
GET  /api/stocks/:symbol/financials  ?year=&quarter=

# 除權息
GET  /api/stocks/:symbol/dividends

# 資料同步（SSE 串流進度）
GET  /api/scraper/stocks          同步股票清單（上市 + 上櫃）
GET  /api/scraper/prices/:symbol  同步單支股票價量歷史
GET  /api/scraper/institution     同步三大法人（指定日期）
GET  /api/scraper/margin          同步融資融券（指定日期）
```

### 4.2 技術指標

技術指標（MA、RSI、MACD、KD、布林通道）**不存入資料庫**，在 API 層依查詢範圍即時運算後回傳，避免資料冗餘與同步問題。

---

## 5. 資料蒐集排程

### 5.1 排程時間表

| 工作 | 時間 | 說明 |
|------|------|------|
| 同步股票清單 | 每日 08:00 | 捕捉新上市/下市異動 |
| 同步日K 價量 | 每日 18:00 | 上市/上櫃收盤資料約 17:30 更新 |
| 同步三大法人 | 每日 18:30 | 交易所 18:00 後公布 |
| 同步融資融券 | 每日 19:00 | 約 18:30 後公布 |
| 同步借券 | 每日 19:30 | |
| 補抓歷史資料 | 手動觸發 | 初次建置時補抓近 5 年歷史 |
| 同步財報 | 每季季末後 45 天 | 配合財報公告期限 |

### 5.2 排程實作選項

- **簡單方式**：Go 內建 `time.Ticker` 搭配 goroutine
- **建議方式**：獨立 `scheduler` service，使用 [robfig/cron](https://github.com/robfig/cron) 套件，以 cron 表達式設定
- **進階**：加入 Redis 做 Job Queue（防止重複執行、支援重試）

---

## 6. 目錄結構

```
34_stock-new/
├── frontend/                         # Nuxt 4
│   ├── app/
│   │   └── pages/
│   │       ├── index.vue             # 股票列表
│   │       └── stocks/
│   │           └── [symbol].vue      # 個股頁面（待建）
│   └── server/routes/api/
│       └── [...path].ts              # SSE-aware 代理
│
├── backend/                          # Go
│   ├── main.go
│   └── internal/
│       ├── config/
│       ├── database/
│       │   └── database.go           # TimescaleDB 初始化
│       ├── models/
│       │   ├── stock.go
│       │   ├── daily_price.go        # 待建
│       │   ├── institutional.go      # 待建
│       │   ├── margin.go             # 待建
│       │   ├── dividend.go           # 待建
│       │   └── financial.go          # 待建
│       ├── handlers/
│       │   ├── stock_handler.go
│       │   ├── price_handler.go      # 待建
│       │   ├── institution_handler.go# 待建
│       │   └── scraper_handler.go
│       ├── scraper/
│       │   ├── twse.go               # 上市股票清單
│       │   ├── tpex.go               # 上櫃股票清單
│       │   ├── prices.go             # 待建：日K
│       │   ├── institution.go        # 待建：三大法人
│       │   ├── margin.go             # 待建：融資融券
│       │   └── dividend.go           # 待建：除權息
│       ├── indicator/
│       │   ├── ma.go                 # 待建：移動平均
│       │   ├── rsi.go                # 待建：RSI
│       │   └── macd.go               # 待建：MACD
│       └── routes/
│           └── routes.go
│
├── docs/
│   └── architecture.md               # 本文件
│
├── .github/
│   └── skills/port-management/
│
├── docker-compose.yml
├── .env.example
└── .gitignore
```

---

## 7. Docker 部署架構

### 7.1 docker-compose.yml 調整

```yaml
services:
  postgres:
    # 從 postgres:16-alpine 改為 timescaledb
    image: timescale/timescaledb:latest-pg16
    container_name: stock_postgres
    environment:
      POSTGRES_USER: ${DB_USER:-postgres}
      POSTGRES_PASSWORD: ${DB_PASS:-postgres}
      POSTGRES_DB: ${DB_NAME:-stockdb}
    ports:
      - "${DB_PORT_EXPOSED:-5432}:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backend/migrations:/docker-entrypoint-initdb.d  # 初始化 SQL
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-postgres}"]
      interval: 10s
      timeout: 5s
      retries: 5

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    container_name: stock_backend
    env_file:
      - .env
    environment:
      PORT: ${BACKEND_INTERNAL_PORT:-8080}
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: ${DB_USER:-postgres}
      DB_PASS: ${DB_PASS:-postgres}
      DB_NAME: ${DB_NAME:-stockdb}
    ports:
      - "${BACKEND_PORT:-8080}:${BACKEND_INTERNAL_PORT:-8080}"
    depends_on:
      postgres:
        condition: service_healthy

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: stock_frontend
    environment:
      NUXT_BACKEND_URL: http://backend:${BACKEND_INTERNAL_PORT:-8080}
    ports:
      - "${FRONTEND_PORT:-3000}:3000"
    depends_on:
      - backend
```

### 7.2 環境變數（.env.example）

```env
# 資料庫
DB_USER=postgres
DB_PASS=postgres
DB_NAME=stockdb

# 對外 Port
FRONTEND_PORT=3000
BACKEND_PORT=8080
DB_PORT_EXPOSED=5432

# 容器內部 Port
BACKEND_INTERNAL_PORT=8080

# Nuxt server-side 後端 URL（Docker 時由 compose 覆蓋）
NUXT_BACKEND_URL=http://localhost:8080
```

---

## 8. 導入優先順序

### Phase 1 — 基礎建設（已完成）

- [x] 股票主檔（上市 + 上櫃同步）
- [x] REST API 基礎架構
- [x] Docker Compose + PostgreSQL
- [x] 前端股票列表 + SSE 同步進度

### Phase 2 — 價量資料

- [ ] 換用 `timescale/timescaledb:latest-pg16` image
- [ ] 建立 `daily_prices` hypertable
- [ ] 建立 `dividends` 表
- [ ] 實作日K 爬取（TWSE + TPEX）
- [ ] 實作還原收盤價計算
- [ ] 建立週K / 月K 連續聚合
- [ ] API：`GET /api/stocks/:symbol/prices`
- [ ] 前端個股頁面 + K 線圖（推薦使用 [lightweight-charts](https://github.com/tradingview/lightweight-charts)）

### Phase 3 — 籌碼資料

- [ ] 建立 `institutional_trades` hypertable
- [ ] 建立 `margin_trading` hypertable
- [ ] 建立 `securities_lending` hypertable
- [ ] 實作三大法人爬取
- [ ] 實作融資融券爬取
- [ ] 實作借券爬取
- [ ] 排程設定（robfig/cron）
- [ ] API：籌碼相關端點
- [ ] 前端籌碼圖表

### Phase 4 — 技術指標

- [ ] 實作 MA（5/10/20/60/120/240 日）
- [ ] 實作 RSI（14 日）
- [ ] 實作 MACD（12/26/9）
- [ ] 實作 KD（9/3/3）
- [ ] 實作布林通道（20 日 ±2σ）
- [ ] API 整合技術指標回傳
- [ ] 前端指標疊加顯示

### Phase 5 — 財報資料（難度最高）

- [ ] 建立 `financials` 表（含 JSONB）
- [ ] 分析公開資訊觀測站 API / HTML 結構
- [ ] 實作季報、年報爬取
- [ ] API：`GET /api/stocks/:symbol/financials`
- [ ] 前端財報頁面

---

## 附錄：欄位對照表

### TWSE 上市日K API 欄位對照

| 原始欄位 | 對應欄位 | 說明 |
|----------|----------|------|
| 日期 | `time` | 轉換為 `YYYY-MM-DD` |
| 開盤價 | `open` | |
| 最高價 | `high` | |
| 最低價 | `low` | |
| 收盤價 | `close` | |
| 成交股數 | `volume` | 原始含逗號，需清理 |
| 成交金額 | `tx_value` | |
| 成交筆數 | `tx_count` | |

### TWSE 三大法人 API 欄位對照

| 原始欄位 | 對應欄位 |
|----------|----------|
| 外陸資買進股數 | `foreign_buy` |
| 外陸資賣出股數 | `foreign_sell` |
| 外陸資淨買進股數 | `foreign_net` |
| 投信買進股數 | `trust_buy` |
| 投信賣出股數 | `trust_sell` |
| 投信淨買進股數 | `trust_net` |
| 自營商買進股數(自行買賣) | `dealer_buy` |
| 自營商賣出股數(自行買賣) | `dealer_sell` |
| 自營商淨買進股數(合計) | `dealer_net` |
