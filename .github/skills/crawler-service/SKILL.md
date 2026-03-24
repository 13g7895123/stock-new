---
name: crawler-service
description: 'Taiwan stock market crawler service written in Go. Use when: implementing new scrapers, debugging data parsing failures, adding broker URLs, understanding OHLCV data flow, tracing fetch errors, modifying worker pool concurrency, troubleshooting TWSE/TPEX API integration, fixing data validation logic, or optimizing PostgreSQL batch insert. Covers broker failover strategy, response text format parsing, ROC/AD date conversion, stock list scraping, Worker Pool architecture, and storage upsert patterns.'
argument-hint: 'Describe the crawler task or issue (e.g., "add new broker", "debug parsing error", "fetch stock list")'
---

# Crawler Service — Taiwan Stock Market (Go)

## When to Use

- Implementing or modifying a scraper for stock daily data (OHLCV)
- Debugging broker response parsing failures
- Adding or changing broker failover URLs
- Understanding the Worker Pool / batch fetch architecture
- Tracing data from HTTP fetch → parse → validate → PostgreSQL upsert
- Integrating TWSE or TPEX stock list API

## Architecture Overview

```
API Request
    │
    ▼
BatchService → WorkerPool (100 goroutines)
    │
    ▼
BrokerManager.FetchWithFailover()
    │  Try Broker1 → fail → Broker2 → ... → success
    ▼
HTTPClient.GetWithRetry()     (fasthttp, max 3 retries, 10 redirects)
    │
    ▼
Parser.ParseBrokerResponse()  (space-split sections, comma-split values)
    │
    ▼
Validator.ValidateBatch()     (10 OHLCV sanity rules)
    │
    ▼
BatchInserter.BatchUpsertWithRetry()  (pgx COPY + ON CONFLICT)
    │
    ▼
PostgreSQL stock_daily_data table
```

## Key Files

| File | Purpose |
|------|---------|
| `internal/scraper/broker.go` | BrokerManager, Failover logic |
| `internal/scraper/client.go` | fasthttp wrapper, redirect & retry |
| `internal/scraper/parser.go` | Response text parser, date conversion |
| `internal/scraper/validator.go` | OHLCV validation rules |
| `internal/scraper/types.go` | DailyData, Broker interface |
| `internal/service/stock_service.go` | Orchestration: fetch → validate → save |
| `internal/service/stock_list_service.go` | TWSE / TPEX stock list scraping |
| `internal/service/batch_service.go` | Batch submit to WorkerPool |
| `internal/worker/pool.go` | Goroutine pool, task queue |
| `internal/worker/task.go` | Task/Result types, status FSM |
| `internal/storage/batch.go` | pgx COPY batch insert |
| `internal/storage/repository.go` | Repository interface + Upsert |
| `internal/storage/models.go` | StockDailyData, Stock DB models |
| `configs/config.yaml` | Broker URLs, worker count, timeouts |

## Quick Reference

- **Add broker URL**: Edit `configs/config.yaml` → `crawler.broker_urls`
- **Response format**: See [./references/data-parsing.md](./references/data-parsing.md)
- **Data structures**: See [./references/data-structures.md](./references/data-structures.md)
- **Broker & stock list URLs**: See [./references/broker-urls.md](./references/broker-urls.md)
- **Architecture detail**: See [./references/architecture.md](./references/architecture.md)

## Step-by-Step: Fetch a Single Stock

1. `POST /api/v1/stocks/{symbol}/daily` triggers `StockService.FetchStockDaily()`
2. `BrokerManager.FetchWithFailover()` iterates broker list; first success wins
3. `HTTPClient.GetWithRetry(url, 3)` — fasthttp GET, auto-follow ≤10 redirects
4. `Parser.ParseBrokerResponse(body, stockCode)` — parse space-delimited text
5. `Validator.ValidateBatch(data)` — discard invalid records silently
6. `BatchInserter.BatchUpsertWithRetry(records, 3)` — pgx COPY + ON CONFLICT
7. Returns `FetchStockDailyResponse{Symbol, Source, RecordCount, Records, Duration}`

## Step-by-Step: Fetch Full Stock List

1. `POST /api/v1/stocks/fetch-all` triggers `StockListService.FetchAllStocks()`
2. `fetchTWSEStocks()` → TWSE API JSON `data[*][0/1]` (上市)
3. `fetchTPEXStocks()` → TPEX API JSON `aaData[*][0/1]` (上櫃)
4. Filter: code must be exactly 4 numeric characters
5. Fallback: use embedded default list if API fails
6. `repository.UpsertStock()` saves each stock to `stocks` table
