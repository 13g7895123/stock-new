package winrate

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"stock-backend/internal/models"

	"gorm.io/gorm"
)

var ErrJobRunning = errors.New("winrate job already running")
var ErrNoSymbols = errors.New("沒有股票可計算（請先同步主力進出資料）")

type workResult struct {
	symbol string
	count  int
	err    error
}

type Runner struct {
	db          *gorm.DB
	concurrency int
	maxHoldDays int
	minNetVol   int

	mu      sync.Mutex
	running bool
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db:          db,
		concurrency: intEnv("WINRATE_CONCURRENCY", 4),
		maxHoldDays: intEnv("WINRATE_MAX_HOLD_DAYS", 120),
		minNetVol:   intEnv("WINRATE_MIN_NET_VOL", 100),
	}
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.WinrateSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": time.Now(),
			"message":      "backend restarted before job completed",
		}).Error
}

// Trigger 觸發批次計算。symbol 為空時計算所有有主力進出資料的股票。
func (r *Runner) Trigger(symbol string) (int, error) {
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
		symbols, err = r.loadSymbols()
	}
	if err != nil {
		r.setRunning(false)
		return 0, err
	}
	if len(symbols) == 0 {
		r.setRunning(false)
		return 0, ErrNoSymbols
	}

	go r.runJob(symbols)
	return len(symbols), nil
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

func (r *Runner) loadSymbols() ([]string, error) {
	var symbols []string
	if err := r.db.Model(&models.MajorBrokerRecord{}).
		Where("side = ? AND days = ?", "buy", 1).
		Distinct().
		Pluck("symbol", &symbols).Error; err != nil {
		return nil, err
	}
	return symbols, nil
}

func (r *Runner) runJob(symbols []string) {
	defer r.setRunning(false)

	job := models.WinrateSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Total:     len(symbols),
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[winrate] create job failed: %v", err)
		return
	}

	jobs := make(chan string)
	results := make(chan workResult, len(symbols))

	wc := r.concurrency
	if wc > len(symbols) {
		wc = len(symbols)
	}
	for i := 0; i < wc; i++ {
		go r.worker(jobs, results)
	}

	go func() {
		defer close(jobs)
		for _, s := range symbols {
			jobs <- s
		}
	}()

	success, fail := 0, 0
	for processed := 1; processed <= len(symbols); processed++ {
		res := <-results
		if res.err == nil {
			success++
		} else {
			fail++
			log.Printf("[winrate][%s] %v", res.symbol, res.err)
		}

		if processed%20 == 0 || processed == len(symbols) {
			msg := fmt.Sprintf("處理中 %d/%d：%s", processed, len(symbols), res.symbol)
			r.db.Model(&models.WinrateSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{"success": success, "fail": fail, "message": msg})
		}
	}

	completedAt := time.Now()
	status := "completed"
	if success == 0 && fail > 0 {
		status = "failed"
	}
	finalMsg := fmt.Sprintf("完成：成功 %d，失敗 %d", success, fail)
	r.db.Model(&models.WinrateSyncJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       status,
			"completed_at": &completedAt,
			"success":      success,
			"fail":         fail,
			"message":      finalMsg,
		})

	log.Printf("[winrate] job %d finished status=%s success=%d fail=%d", job.ID, status, success, fail)
}

func (r *Runner) worker(jobs <-chan string, results chan<- workResult) {
	for symbol := range jobs {
		n, err := CalculateSymbol(r.db, symbol, r.maxHoldDays, r.minNetVol)
		results <- workResult{symbol: symbol, count: n, err: err}
	}
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
