# Architecture Deep Dive

## Worker Pool (`internal/worker/pool.go`)

### Concurrency Model

```
BatchService.BatchUpdateStocks()
    │  for each symbol
    ▼
WorkerPool.Submit(StockFetchTask)
    │  non-blocking; returns error if queue full
    ▼
taskQueue (buffered chan, size = max_workers * 2)
    │
    ▼  (goroutines)
worker(id) × N  ← N = max_workers (default: 100)
    │  context timeout: 2 minutes per task
    ▼
executeTask() → StockFetcher.FetchStockDaily()
    │
    ▼
resultQueue (buffered chan)
    │
    ▼
resultProcessor() goroutine
    │  updates task.Status, logs result
    ▼
tasks map[string]*StockFetchTask  (protected by sync.RWMutex)
```

### Task Status FSM

```
pending ──► running ──► completed
                    └──► failed
```

### Key Config (`configs/config.yaml`)

```yaml
crawler:
  max_workers: 100       # goroutines in pool
  batch_size: 50         # records per DB batch
  request_timeout: 30s   # per HTTP request
  retry_count: 3         # per broker attempt
  smart_skip_days: 1     # skip if data exists within N days
```

---

## HTTP Layer (`internal/scraper/client.go`)

- **Library:** `github.com/valyala/fasthttp`
- **Connections per host:** up to 1000
- **Idle connection duration:** 10s
- **Redirect handling:** recursive, max 10 hops
  - Relative redirects resolved to absolute using original scheme+host
- **Retry loop:** `GetWithRetry(ctx, url, retryCount)` — checks `ctx.Done()` between attempts

---

## Broker Failover (`internal/scraper/broker.go`)

```go
func (bm *BrokerManager) FetchWithFailover(ctx context.Context, symbol string) (*FetchResult, error) {
    for _, broker := range bm.brokers {
        data, err := broker.FetchDailyData(ctx, symbol)
        if err != nil {
            continue   // try next broker
        }
        return &FetchResult{...success...}, nil
    }
    return nil, fmt.Errorf("failed to fetch from all brokers: %w", lastErr)
}
```

`FetchFromAll()` fetches from **all** brokers simultaneously (used for data comparison/auditing).

---

## Storage Layer

### Upsert Strategy (`internal/storage/repository.go`)

```sql
INSERT INTO stock_daily_data (stock_code, trade_date, open_price, ...)
VALUES (...)
ON CONFLICT (stock_code, trade_date)
DO UPDATE SET
    open_price  = EXCLUDED.open_price,
    high_price  = EXCLUDED.high_price,
    low_price   = EXCLUDED.low_price,
    close_price = EXCLUDED.close_price,
    volume      = EXCLUDED.volume,
    updated_at  = NOW()
```

### High-Speed Batch Insert (`internal/storage/batch.go`)

Uses **PostgreSQL COPY protocol** via `pgx.CopyFrom` — 10–100× faster than individual INSERTs.

```go
conn.Conn().CopyFrom(
    ctx,
    pgx.Identifier{"stock_daily_data"},
    []string{"stock_code", "trade_date", "open_price", ...},
    pgx.CopyFromRows(rows),
)
```

`BatchUpsertWithRetry(ctx, records, maxRetries=3)`:
1. Attempt COPY insert
2. On conflict: fall back to individual upsert per record
3. Retry up to `maxRetries` times on transient errors

### Connection Pool (`internal/storage/postgres.go` + `batch.go`)

| Pool          | Driver  | max_open | max_idle |
|---------------|---------|----------|----------|
| sqlx (queries)| `lib/pq` | 20      | 10       |
| pgxpool (COPY)| `pgx/v5` | 20      | 5        |

---

## API Endpoints (`internal/api/router.go`)

| Method | Path | Handler |
|--------|------|---------|
| GET | `/health` | `HealthHandler.HealthCheck` |
| GET | `/api/v1/stocks/{symbol}/daily` | `StockHandler.GetStockDaily` |
| GET | `/api/v1/stocks/{symbol}/history` | `StockHandler.GetStockHistory` |
| GET | `/api/v1/stocks/{symbol}/latest` | `StockHandler.GetLatestData` |
| POST | `/api/v1/stocks/batch-update` | `BatchHandler.BatchUpdate` |
| POST | `/api/v1/stocks/fetch-all` | `StockListHandler.FetchAllStocks` |
| GET | `/api/v1/batch/{batch_id}` | `BatchHandler.GetBatchStatus` |
| GET | `/api/v1/workers/stats` | `BatchHandler.GetWorkerStats` |
| GET | `/api/v1/stats/stocks-summary` | `StatsHandler.GetStocksSummary` |

Middleware stack (outer → inner): Recovery → Logging → CORS → Mux

---

## Observability

- **Metrics:** Prometheus endpoint `/metrics` on port 9090 (toggled by `metrics.enabled`)
- **Logging:** `go.uber.org/zap`, JSON format, configurable level
- **Log file:** `crawler.log` / `nohup.out` in service root
- **Grafana dashboards:** see `deployments/grafana/`
