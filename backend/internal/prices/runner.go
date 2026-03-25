package prices

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"stock-backend/internal/models"
	"stock-backend/internal/scraper"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrJobRunning = errors.New("price sync job already running")
var ErrNoSymbols = errors.New("沒有股票需要爬取（請先同步股票清單）")

type stockInfo struct {
	Symbol string
	Market string
}

type fetchResult struct {
	symbol  string
	records int
	err     error
}

type Runner struct {
	db          *gorm.DB
	concurrency int

	mu      sync.Mutex
	running bool
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db:          db,
		concurrency: intEnv("PRICE_SYNC_CONCURRENCY", 3),
	}
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.PriceSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": time.Now(),
			"message":      "backend restarted before job completed",
		}).Error
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) Trigger() (int, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return 0, ErrJobRunning
	}
	r.running = true
	r.mu.Unlock()

	stocks, err := r.loadStocks()
	if err != nil {
		r.setRunning(false)
		return 0, err
	}
	if len(stocks) == 0 {
		r.setRunning(false)
		return 0, ErrNoSymbols
	}

	go r.runJob(stocks)
	return len(stocks), nil
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

func (r *Runner) loadStocks() ([]stockInfo, error) {
	var rows []struct {
		Symbol string
		Market string
	}
	// 只取四碼、非零開頭的一般股票（過濾 ETF、權證、特別股等）
	if err := r.db.Model(&models.Stock{}).
		Select("symbol, market").
		Where("symbol ~ ?", `^[1-9][0-9]{3}$`).
		Order("symbol ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]stockInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, stockInfo{Symbol: row.Symbol, Market: row.Market})
	}
	return out, nil
}

func (r *Runner) runJob(stocks []stockInfo) {
	defer r.setRunning(false)

	job := models.PriceSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Total:     len(stocks),
		Success:   0,
		Fail:      0,
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[price-sync] create job failed: %v", err)
		return
	}

	type workItem = stockInfo
	jobs := make(chan workItem)
	results := make(chan fetchResult, len(stocks))
	workerCount := r.concurrency
	if workerCount > len(stocks) {
		workerCount = len(stocks)
	}
	for i := 0; i < workerCount; i++ {
		go r.worker(jobs, results)
	}

	go func() {
		defer close(jobs)
		for _, s := range stocks {
			jobs <- s
		}
	}()

	success := 0
	fail := 0
	for processed := 1; processed <= len(stocks); processed++ {
		res := <-results
		if res.err == nil {
			success++
		} else {
			fail++
			log.Printf("[price-sync][%s] %v", res.symbol, res.err)
		}

		if processed%10 == 0 || processed == len(stocks) {
			msg := fmt.Sprintf("處理中 %d/%d：%s", processed, len(stocks), res.symbol)
			if err := r.db.Model(&models.PriceSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{
					"success": success,
					"fail":    fail,
					"message": msg,
				}).Error; err != nil {
				log.Printf("[price-sync] update progress failed: %v", err)
			}
		}
	}

	completedAt := time.Now()
	status := "completed"
	if success == 0 && fail > 0 {
		status = "failed"
	}
	finalMsg := fmt.Sprintf("完成：成功 %d，失敗 %d", success, fail)
	if err := r.db.Model(&models.PriceSyncJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       status,
			"completed_at": &completedAt,
			"success":      success,
			"fail":         fail,
			"message":      finalMsg,
		}).Error; err != nil {
		log.Printf("[price-sync] finalize job failed: %v", err)
	}

	log.Printf("[price-sync] job %d finished status=%s success=%d fail=%d", job.ID, status, success, fail)
}

func (r *Runner) worker(jobs <-chan stockInfo, results chan<- fetchResult) {
	for s := range jobs {
		n, err := r.fetchAllHistory(s)
		results <- fetchResult{symbol: s.Symbol, records: n, err: err}
		// 每支股票之間稍作間隔，避免對交易所 API rate limit
		time.Sleep(200 * time.Millisecond)
	}
}

// fetchAllHistory 先嘗試券商 API（一次取全部歷史），失敗才回退到 TWSE/TPEX 月份迴圈
func (r *Runner) fetchAllHistory(s stockInfo) (int, error) {
	// ── 方案一：券商 API（一次請求取全部） ──────────────────────
	brokerResult, brokerErr := scraper.FetchBrokerStockHistory(s.Symbol)
	if brokerErr == nil && len(brokerResult.Records) > 30 {
		log.Printf("[price-sync][%s] broker OK source=%s records=%d", s.Symbol, brokerResult.Source, len(brokerResult.Records))
		return r.saveRecords(brokerResult.Records)
	}
	if brokerErr != nil {
		log.Printf("[price-sync][%s] broker failed, fallback to TWSE/TPEX: %v", s.Symbol, brokerErr)
	} else {
		log.Printf("[price-sync][%s] broker returned only %d records, fallback to TWSE/TPEX", s.Symbol, len(brokerResult.Records))
	}

	// ── 方案二：TWSE/TPEX 逐月迴圈 ─────────────────────────────
	return r.fetchAllHistoryByMonth(s)
}

// fetchAllHistoryByMonth 從當月往回逐月抓，連續 3 個月無資料即停止
func (r *Runner) fetchAllHistoryByMonth(s stockInfo) (int, error) {
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	var all []models.DailyPrice
	emptyStreak := 0
	const maxEmptyStreak = 3

	for i := 0; ; i++ {
		t := now.AddDate(0, -i, 0)
		ym := fmt.Sprintf("%d%02d", t.Year(), t.Month())

		var records []models.DailyPrice
		var fetchErr error
		if s.Market == "TWSE" {
			records, fetchErr = scraper.FetchTWSEStockHistory(s.Symbol, ym)
		} else {
			records, fetchErr = scraper.FetchTPEXStockHistory(s.Symbol, ym)
		}

		if fetchErr != nil || len(records) == 0 {
			if fetchErr != nil {
				log.Printf("[price-sync][%s] fetch %s error: %v", s.Symbol, ym, fetchErr)
			}
			emptyStreak++
			if emptyStreak >= maxEmptyStreak {
				break
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}

		emptyStreak = 0
		all = append(all, records...)
		time.Sleep(120 * time.Millisecond)
	}

	return r.saveRecords(all)
}

// saveRecords UPSERT 一組 DailyPrice 至資料庫
func (r *Runner) saveRecords(all []models.DailyPrice) (int, error) {
	if len(all) == 0 {
		return 0, nil
	}
	result := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "tx_value", "tx_count"}),
	}).CreateInBatches(&all, 500)

	if result.Error != nil {
		return 0, result.Error
	}
	return len(all), nil
}

// FetchSingle 同步爬取單支股票的全部歷史日K，用於測試或手動補抓
// useBroker=true 只用券商 API；false 只用 TWSE/TPEX 月份迴圈；nil/未傳使用自動（先券商再 fallback）
func (r *Runner) FetchSingle(symbol, market string) (int, error) {
	return r.fetchAllHistory(stockInfo{Symbol: symbol, Market: market})
}

// FetchSingleBrokerOnly 只透過券商 API 抓取（不走 TWSE/TPEX 月份迴圈）
func (r *Runner) FetchSingleBrokerOnly(symbol string) (*scraper.BrokerFetchResult, error) {
	return scraper.FetchBrokerStockHistory(symbol)
}

func intEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}
