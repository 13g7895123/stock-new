# 券商勝率分析功能規劃

## 一、功能目標

針對每一檔股票，分析各券商分點的「買賣決策勝率」。  
判斷依據：若某券商在低點大量建倉，並在後續更高點出現賣出紀錄，視為一次「勝局」；反之為「敗局」。  
藉此找出長期在特定股票中具備優勢的「潛在主力券商」。

---

## 二、資料現況

| 資料表 | 說明 |
|--------|------|
| `major_broker_records` | 每支股票每日（1/5/10/20/40/60日）主力進出，含 `broker_name`、`buy_vol`、`sell_vol`、`net_vol`、`side`、`data_date` |
| `daily_prices` | 每日 OHLCV，含 `close` |

### 限制說明

目前 `major_broker_records` 的資料結構僅保存**每日排行前 10 名**的買超 / 賣超券商，**不含完整逐日每家券商的買賣量**。  
因此勝率計算必須以「有出現在排行榜上」作為信號，而非完整連續部位。

---

## 三、勝率定義

### 核心概念：事件配對（Pairing）

一次「交易事件」由以下兩個信號組成：

| 事件 | 條件 |
|------|------|
| 建倉信號（Entry） | 某券商在 N 日期出現在**買超榜** (`side=buy`)，且 `net_vol` 達到門檻 |
| 出場信號（Exit） | 同一券商在**建倉後 M 天內**出現在**賣超榜** (`side=sell`) |

### 勝率計算

```
勝率 = 獲利出場次數 / 總出場次數

獲利出場定義：
  exit_price > entry_price
  即：出場日 close > 建倉日 close
```

### 延伸指標

| 指標 | 說明 |
|------|------|
| 建倉次數 | 出現買超信號的總次數 |
| 已出場次數 | 有後續賣超配對的次數 |
| 勝率 | 獲利出場 / 已出場 |
| 平均報酬率 | `mean((exit_close - entry_close) / entry_close)` |
| 平均持倉天數 | entry → exit 的平均天數 |
| 最大單次獲利 | 單次最高報酬率 |

---

## 四、資料庫設計

### 新增資料表：`broker_trade_events`

```sql
CREATE TABLE broker_trade_events (
    id              BIGSERIAL PRIMARY KEY,
    symbol          VARCHAR(10)    NOT NULL,
    broker_name     VARCHAR(60)    NOT NULL,
    entry_date      DATE           NOT NULL,
    entry_close     NUMERIC(10,2)  NOT NULL,  -- 建倉日收盤價
    entry_net_vol   INTEGER        NOT NULL,  -- 建倉日淨買量（張）
    exit_date       DATE,                      -- NULL 表示尚未出場
    exit_close      NUMERIC(10,2),             -- 出場日收盤價
    exit_net_vol    INTEGER,                   -- 出場日淨賣量（張）
    return_pct      NUMERIC(8,4),              -- (exit_close - entry_close) / entry_close * 100
    holding_days    INTEGER,                   -- exit_date - entry_date
    is_win          BOOLEAN,                   -- exit_close > entry_close
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    UNIQUE (symbol, broker_name, entry_date)
);

CREATE INDEX idx_bte_symbol        ON broker_trade_events(symbol);
CREATE INDEX idx_bte_broker        ON broker_trade_events(broker_name);
CREATE INDEX idx_bte_entry_date    ON broker_trade_events(entry_date);
CREATE INDEX idx_bte_exit_date     ON broker_trade_events(exit_date);
```

### 新增 Materialized View：`broker_winrate_summary`

```sql
CREATE MATERIALIZED VIEW broker_winrate_summary AS
SELECT
    symbol,
    broker_name,
    COUNT(*)                                                         AS total_entries,
    COUNT(*) FILTER (WHERE exit_date IS NOT NULL)                   AS total_exits,
    COUNT(*) FILTER (WHERE is_win = TRUE)                           AS win_count,
    ROUND(
        COUNT(*) FILTER (WHERE is_win = TRUE)::numeric
        / NULLIF(COUNT(*) FILTER (WHERE exit_date IS NOT NULL), 0) * 100,
        2
    )                                                                AS win_rate_pct,
    ROUND(AVG(return_pct) FILTER (WHERE exit_date IS NOT NULL), 2)  AS avg_return_pct,
    ROUND(AVG(holding_days) FILTER (WHERE exit_date IS NOT NULL), 1) AS avg_holding_days,
    MAX(return_pct)                                                  AS max_return_pct,
    MAX(entry_date)                                                  AS last_entry_date
FROM broker_trade_events
GROUP BY symbol, broker_name
WITH DATA;

CREATE UNIQUE INDEX ON broker_winrate_summary(symbol, broker_name);
```

> `REFRESH MATERIALIZED VIEW CONCURRENTLY broker_winrate_summary;` 每次事件表更新後執行。

---

## 五、後端架構

### 5.1 目錄結構（新增）

```
backend/internal/
├── winrate/
│   ├── runner.go          # 批次計算勝率的 Runner
│   └── calculator.go      # 事件配對邏輯
├── models/
│   └── winrate.go         # BrokerTradeEvent, BrokerWinrateSummary model
└── handlers/
    └── winrate_handler.go # REST API handler
```

### 5.2 Go Model

```go
// models/winrate.go

type BrokerTradeEvent struct {
    ID           uint      `gorm:"primaryKey;autoIncrement"`
    Symbol       string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_bte_unique"`
    BrokerName   string    `gorm:"type:varchar(60);not null;uniqueIndex:idx_bte_unique"`
    EntryDate    time.Time `gorm:"type:date;not null;uniqueIndex:idx_bte_unique"`
    EntryClose   float64   `gorm:"type:numeric(10,2);not null"`
    EntryNetVol  int       `gorm:"not null"`
    ExitDate     *time.Time `gorm:"type:date"`
    ExitClose    *float64  `gorm:"type:numeric(10,2)"`
    ExitNetVol   *int
    ReturnPct    *float64  `gorm:"type:numeric(8,4)"`
    HoldingDays  *int
    IsWin        *bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (BrokerTradeEvent) TableName() string { return "broker_trade_events" }

type BrokerWinrateSummary struct {
    Symbol          string   `gorm:"primaryKey"`
    BrokerName      string   `gorm:"primaryKey"`
    TotalEntries    int
    TotalExits      int
    WinCount        int
    WinRatePct      float64
    AvgReturnPct    float64
    AvgHoldingDays  float64
    MaxReturnPct    float64
    LastEntryDate   time.Time
}

func (BrokerWinrateSummary) TableName() string { return "broker_winrate_summary" }
```

### 5.3 計算流程（calculator.go）

```
對每一檔股票
  └─ 取出所有 side=buy 的 major_broker_records
        按 (broker_name, data_date) 排序
  └─ 對每筆買超記錄（entry）
        在 daily_prices 取 entry_date 的 close → entry_close
        查詢同一 broker_name 在 entry_date 後 MAX_HOLD_DAYS 內
        有無出現 side=sell 的紀錄
          ├─ 有 → 取最近一筆 → exit_date、exit_close
          │       計算 return_pct、holding_days、is_win
          └─ 無 → exit_date = NULL（尚未出場，不計入勝率）
  └─ UPSERT broker_trade_events
  └─ REFRESH MATERIALIZED VIEW broker_winrate_summary
```

### 5.4 可調整參數（環境變數）

| 變數 | 預設 | 說明 |
|------|------|------|
| `WINRATE_MAX_HOLD_DAYS` | `120` | 建倉後最多幾天內找出場信號 |
| `WINRATE_MIN_NET_VOL` | `100` | 買超量門檻（張），小於此值不計入 |
| `WINRATE_MIN_ENTRIES` | `3` | 勝率統計最少需幾次建倉記錄才顯示 |
| `WINRATE_CONCURRENCY` | `4` | 並行計算的 goroutine 數量 |

---

## 六、REST API 設計

### 6.1 觸發計算

```
POST /api/winrate/trigger
Body: { "symbol": "2330" }   // 如需僅計算單一股票
      {}                      // 空 body = 全股計算
```

```json
// Response 200
{
  "queued": 1800,
  "message": "已開始計算券商勝率"
}
```

### 6.2 查詢某股票的勝率排行

```
GET /api/stocks/:symbol/broker-winrate?min_entries=5&sort=win_rate_pct&order=desc
```

```json
// Response 200
[
  {
    "broker_name": "元大-中山",
    "total_entries": 12,
    "total_exits": 10,
    "win_count": 8,
    "win_rate_pct": 80.00,
    "avg_return_pct": 23.45,
    "avg_holding_days": 32.5,
    "max_return_pct": 67.80,
    "last_entry_date": "2026-01-15"
  }
]
```

### 6.3 查詢某券商的跨股表現

```
GET /api/broker-winrate?broker_name=元大-中山&min_entries=3&limit=20
```

```json
// Response 200
[
  {
    "symbol": "2330",
    "broker_name": "元大-中山",
    "win_rate_pct": 80.00,
    "avg_return_pct": 23.45,
    ...
  }
]
```

### 6.4 查詢單一股票某券商的歷史交易事件

```
GET /api/stocks/:symbol/broker-winrate/:broker_name/events
```

```json
// Response 200
[
  {
    "entry_date": "2025-03-10",
    "entry_close": 30.00,
    "entry_net_vol": 500,
    "exit_date": "2025-08-20",
    "exit_close": 100.00,
    "return_pct": 233.33,
    "holding_days": 163,
    "is_win": true
  }
]
```

---

## 七、前端規劃

### 7.1 股票詳情頁新增 Tab「券商勝率」

| 欄位 | 說明 |
|------|------|
| 券商名稱 | |
| 建倉次數 | |
| 已出場次數 | |
| 勝率 % | 色塊標示：>=70% 綠、40-70% 黃、<40% 紅 |
| 平均報酬率 % | |
| 平均持倉天數 | |
| 最高單次報酬率 % | |
| 最近建倉日 | |

### 7.2 點擊券商後，展開「歷史交易事件」列表

- 時間軸式呈現：建倉 → 出場
- 以顏色區分獲利（綠）/ 虧損（紅）/ 尚未出場（灰）

### 7.3 「跨股勝率」頁面（選配）

- 搜尋特定券商，看其在哪些股票的勝率最高
- 可作為主力偏好股票的提示

---

## 八、實作路徑（建議順序）

```
Phase 1：資料層
  ☐ 新增 models/winrate.go（BrokerTradeEvent, BrokerWinrateSummary）
  ☐ DB Migration：建立 broker_trade_events 資料表
  ☐ DB Migration：建立 broker_winrate_summary materialized view

Phase 2：計算邏輯
  ☐ winrate/calculator.go：事件配對核心邏輯
  ☐ 撰寫單元測試，驗證配對與勝率計算正確性
  ☐ winrate/runner.go：批次 Runner（仿照 prices/runner.go 架構）

Phase 3：API 層
  ☐ winrate_handler.go：Trigger、GetBySymbol、GetBrokerEvents 三支 API
  ☐ routes/routes.go 新增路由

Phase 4：前端
  ☐ 股票詳情頁新增「券商勝率」Tab
  ☐ 歷史交易事件展開列表
```

---

## 九、注意事項與限制

### 資料品質

- `major_broker_records` 僅保存**每日排行前 10 名**，因此勝率是基於「夠強力才上榜」的信號，樣本存在選擇偏誤（上榜本身就代表大量進出）。
- 同一券商同一天若同時出現買超與賣超，代表不同標的混排可能，計算時以 `side` 區分。

### 信號雜訊

- 短天期（如 1 日）資料波動較大，建議計算勝率時優先使用 `days=5` 或 `days=10` 的記錄，信號更穩定。
- 可新增 `days` 作為計算時的篩選條件（`WINRATE_SIGNAL_DAYS` 環境變數）。

### 計算觸發時機

建議在主力進出批次爬取（`POST /api/major/trigger`）完成後，自動觸發勝率重新計算，可在 `major/runner.go` 的 `runJob` 完成時呼叫 `winrate.Runner.Trigger()`。
