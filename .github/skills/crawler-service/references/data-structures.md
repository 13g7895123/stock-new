# Data Structures

## Scraper Layer (`internal/scraper/types.go`)

### `DailyData` — Parsed result from broker response

```go
type DailyData struct {
    StockCode   string    // e.g. "2330"
    TradeDate   time.Time // UTC 00:00:00
    OpenPrice   float64
    HighPrice   float64
    LowPrice    float64
    ClosePrice  float64
    Volume      int64     // shares traded
    Turnover    float64   // optional, may be 0
    DataSource  string    // always "go_broker_crawler"
    DataQuality string    // always "corrected_daily"
}
```

### `Broker` — Interface each broker URL must satisfy

```go
type Broker interface {
    FetchDailyData(ctx context.Context, symbol string) ([]DailyData, error)
    Name() string
    HealthCheck(ctx context.Context) error
}
```

### `FetchResult` — Single broker fetch outcome

```go
type FetchResult struct {
    Symbol      string
    Data        []DailyData
    Source      string        // broker name
    Success     bool
    Error       error
    Duration    time.Duration
    RecordCount int
}
```

### `BatchResult` — Aggregate of batch operation

```go
type BatchResult struct {
    TotalProcessed int
    SuccessCount   int
    ErrorCount     int
    SkippedCount   int
    Duration       time.Duration
    Errors         []BatchError  // { Symbol, Source, Error, Time }
}
```

---

## Storage Layer (`internal/storage/models.go`)

### `StockDailyData` — maps to `stock_daily_data` PostgreSQL table

```go
type StockDailyData struct {
    ID              int64      // PK, auto-increment
    StockCode       string     // "2330"
    TradeDate       time.Time
    OpenPrice       *float64   // Nullable
    HighPrice       *float64   // Nullable
    LowPrice        *float64   // Nullable
    ClosePrice      *float64   // Nullable
    Volume          *int64     // Nullable
    Turnover        *float64   // Nullable
    PriceChange     *float64   // Computed, Nullable
    PriceChangeRate *float64   // Computed, Nullable
    MA5             *float64   // Computed, Nullable
    MA10            *float64   // Computed, Nullable
    MA20            *float64   // Computed, Nullable
    DataSource      string     // "go_broker_crawler"
    DataQuality     *string    // "corrected_daily"
    IsValidated     bool
    CreatedAt       time.Time
    UpdatedAt       *time.Time // Nullable
}
```

### `Stock` — maps to `stocks` table

```go
type Stock struct {
    ID               int64
    StockCode        string     // "2330"
    StockName        string     // "台積電"
    Market           string     // "上市" or "上櫃"
    Industry         *string    // Nullable
    CapitalStock     *int64     // Nullable
    CapitalUpdatedAt *time.Time // Nullable
    IsActive         bool
    CreatedAt        time.Time
    UpdatedAt        *time.Time // Nullable
}
```

### `TaskExecutionLog` — maps to `task_execution_logs` table

```go
type TaskExecutionLog struct {
    ID              int64
    TaskName        string
    TaskType        string
    Parameters      *string
    Status          string    // "pending" | "running" | "completed" | "failed"
    StartTime       time.Time
    EndTime         *time.Time
    DurationSeconds *float64
    Progress        int
    ProcessedCount  int
    TotalCount      int
    SuccessCount    int
    ErrorCount      int
    ResultSummary   *string
    ErrorMessage    *string
    CreatedBy       *string
}
```

---

## Service Layer Response Types

### `FetchStockDailyResponse` (stock_service.go)

```go
type FetchStockDailyResponse struct {
    Symbol      string                   // "2330"
    Source      string                   // broker name
    RecordCount int
    Records     []storage.StockDailyData
    Duration    time.Duration
    FetchedAt   time.Time
}
```

### `TWStockInfo` (stock_list_service.go)

```go
type TWStockInfo struct {
    Code     string  // 4-digit numeric
    Name     string
    Market   string  // "上市" or "上櫃"
    Type     string  // "股票"
    Industry string
}
```

---

## Validation Rules (`internal/scraper/validator.go`)

All 10 rules must pass; any failure silently discards the record:

| Rule | Condition |
|------|-----------|
| 1    | `stock_code != ""` |
| 2    | `trade_date` not zero |
| 3    | `open > 0` |
| 4    | `high > 0` |
| 5    | `low > 0` |
| 6    | `close > 0` |
| 7    | `high >= open` |
| 8    | `high >= close` |
| 9    | `high >= low` |
| 10   | `low <= open` AND `low <= close` |
| 11   | `volume >= 0` |
