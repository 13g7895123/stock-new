package major

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

var ErrJobRunning = errors.New("major job already running")
var ErrNoSymbols = errors.New("沒有股票需要爬取（請先同步股票清單）")

type Runner struct {
	db           *gorm.DB
	concurrency  int
	requestDelay time.Duration

	mu      sync.Mutex
	running bool
}

type scrapeResult struct {
	symbol  string
	success bool
	err     error
	records int
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db:           db,
		concurrency:  intEnv("MAJOR_CONCURRENCY", 5),
		requestDelay: durationEnv("MAJOR_REQUEST_DELAY", 200*time.Millisecond),
	}
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.MajorSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": time.Now(),
			"message":      "backend restarted before job completed",
		}).Error
}

// Trigger 非同步觸發批次爬取。
// symbol 非空時只爬單支；days ≤ 0 時預設為 1。
func (r *Runner) Trigger(symbol string, days int) (int, error) {
	if days <= 0 {
		days = 1
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return 0, ErrJobRunning
	}
	r.running = true
	r.mu.Unlock()

	var symbols []string
	var err error
	if symbol != "" {
		symbols = []string{symbol}
	} else {
		symbols, err = r.loadSymbols(days)
	}
	if err != nil {
		r.setRunning(false)
		return 0, err
	}
	if len(symbols) == 0 {
		r.setRunning(false)
		return 0, ErrNoSymbols
	}

	go r.runJob(symbols, days)
	return len(symbols), nil
}

// FetchSingle 同步爬取單支股票，直接回傳結果（用於 API 測試）。
func (r *Runner) FetchSingle(symbol string, days int) (*scraper.MajorFetchResult, error) {
	if days <= 0 {
		days = 1
	}
	result, err := scraper.FetchMajorBrokers(symbol, days)
	if err != nil {
		return result, err
	}
	if len(result.Records) > 0 {
		if err := r.saveRecords(0, result.Records); err != nil {
			return result, fmt.Errorf("save records failed: %w", err)
		}
	}
	return result, nil
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

// loadSymbols 回傳今日（指定 days）尚未爬取的股票清單。
func (r *Runner) loadSymbols(days int) ([]string, error) {
	today := todayDate()
	var symbols []string
	if err := r.db.Raw(
		`SELECT s.symbol
		 FROM stocks s
		 WHERE s.symbol <> ''
		   AND NOT EXISTS (
		     SELECT 1 FROM major_broker_records m
		     WHERE m.symbol = s.symbol
		       AND m.data_date = ?
		       AND m.days = ?
		   )
		 ORDER BY s.symbol ASC`,
		today, days,
	).Scan(&symbols).Error; err != nil {
		return nil, err
	}
	return symbols, nil
}

func (r *Runner) runJob(symbols []string, days int) {
	defer r.setRunning(false)

	job := models.MajorSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Days:      days,
		Total:     len(symbols),
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[major] create job failed: %v", err)
		return
	}

	jobs := make(chan string)
	results := make(chan scrapeResult, len(symbols))

	workerCount := min(r.concurrency, len(symbols))
	for i := 0; i < workerCount; i++ {
		go r.worker(job.ID, days, jobs, results)
	}

	go func() {
		defer close(jobs)
		for _, sym := range symbols {
			jobs <- sym
		}
	}()

	success := 0
	fail := 0
	const maxFailureSamples = 20
	failureSamples := make([]string, 0, maxFailureSamples)

	for processed := 1; processed <= len(symbols); processed++ {
		res := <-results
		if res.success {
			success++
		} else {
			fail++
			errMsg := "unknown error"
			if res.err != nil {
				errMsg = res.err.Error()
			}
			log.Printf("[major][%s] %v", res.symbol, errMsg)
			if len(failureSamples) < maxFailureSamples {
				failureSamples = append(failureSamples, fmt.Sprintf("%s: %s", res.symbol, errMsg))
			}
		}

		if processed%5 == 0 || processed == len(symbols) {
			msg := fmt.Sprintf("處理中 %d/%d：%s（記錄 %d 筆）", processed, len(symbols), res.symbol, res.records)
			if err := r.db.Model(&models.MajorSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{"success": success, "fail": fail, "message": msg}).Error; err != nil {
				log.Printf("[major] update progress failed: %v", err)
			}
		}
	}

	completedAt := time.Now()
	status := "completed"
	if success == 0 && fail > 0 {
		status = "failed"
	}

	message := fmt.Sprintf("完成：成功 %d，失敗 %d（days=%d）", success, fail, days)
	if len(failureSamples) > 0 {
		message += "\n\n失敗範例：\n"
		for _, s := range failureSamples {
			message += "• " + s + "\n"
		}
		if fail > len(failureSamples) {
			message += fmt.Sprintf("…共 %d 支失敗", fail)
		}
	}

	if err := r.db.Model(&models.MajorSyncJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       status,
			"completed_at": &completedAt,
			"success":      success,
			"fail":         fail,
			"message":      message,
		}).Error; err != nil {
		log.Printf("[major] finalize job failed: %v", err)
	}

	log.Printf("[major] job %d finished status=%s success=%d fail=%d days=%d", job.ID, status, success, fail, days)
}

func (r *Runner) worker(jobID uint, days int, jobs <-chan string, results chan<- scrapeResult) {
	for sym := range jobs {
		ok, n, err := r.scrapeSymbol(jobID, sym, days)
		results <- scrapeResult{symbol: sym, success: ok, err: err, records: n}
		if r.requestDelay > 0 {
			time.Sleep(r.requestDelay)
		}
	}
}

func (r *Runner) scrapeSymbol(jobID uint, symbol string, days int) (bool, int, error) {
	result, err := scraper.FetchMajorBrokers(symbol, days)
	if err != nil {
		return false, 0, err
	}
	// 無資料（當日無主力進出）視為成功，不儲存任何記錄
	if len(result.Records) == 0 {
		return true, 0, nil
	}
	for i := range result.Records {
		result.Records[i].JobID = jobID
	}
	if err := r.saveRecords(jobID, result.Records); err != nil {
		return false, 0, err
	}
	return true, len(result.Records), nil
}

func (r *Runner) saveRecords(jobID uint, records []models.MajorBrokerRecord) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "symbol"},
			{Name: "data_date"},
			{Name: "days"},
			{Name: "side"},
			{Name: "rank"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"job_id", "broker_name", "buy_vol", "sell_vol", "net_vol", "pct", "scraped_at",
		}),
	}).Create(&records).Error
}

// ─── 工具函式 ───────────────────────────────────────────────────────────────

func todayDate() time.Time {
	taipei := time.FixedZone("Asia/Taipei", 8*3600)
	now := time.Now().In(taipei)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, taipei)
}

func intEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func durationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
