package institutional

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

var ErrJobRunning = errors.New("institutional job already running")

type Runner struct {
	db           *gorm.DB
	requestDelay time.Duration

	mu      sync.Mutex
	running bool
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db:           db,
		requestDelay: durationEnv("INSTITUTIONAL_REQUEST_DELAY", 1500*time.Millisecond),
	}
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.InstitutionalSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": time.Now(),
			"message":      "backend restarted before job completed",
		}).Error
}

// Trigger 非同步觸發批次爬取，days 指定要補幾天（1 = 只補今日）。
// 因兩個 API（TWSE + TPEX）都是「一次傳回全部個股」，
// 所以每天只需發 2 個請求，total 代表日期數量。
func (r *Runner) Trigger(days int) (int, error) {
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

	dates := buildTradingDates(days)

	go r.runJob(dates)
	return len(dates), nil
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

// buildTradingDates 回傳從今日往前 days 天的日曆日（含今日）。
// 非交易日的情況由 API 回傳空資料來處理，此處不做過濾。
func buildTradingDates(days int) []time.Time {
	taipei := time.FixedZone("Asia/Taipei", 8*3600)
	now := time.Now().In(taipei)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, taipei)

	result := make([]time.Time, 0, days)
	for i := 0; i < days; i++ {
		result = append(result, today.AddDate(0, 0, -i))
	}
	return result
}

func (r *Runner) runJob(dates []time.Time) {
	defer r.setRunning(false)

	job := models.InstitutionalSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Total:     len(dates),
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[institutional] create job failed: %v", err)
		return
	}

	success := 0
	fail := 0
	failSamples := make([]string, 0, 10)

	for i, date := range dates {
		dateStr := date.Format("2006-01-02")
		twseRecs, err := scraper.FetchTWSEInstitutional(date)
		if err != nil {
			fail++
			log.Printf("[institutional] TWSE %s failed: %v", dateStr, err)
			if len(failSamples) < 10 {
				failSamples = append(failSamples, fmt.Sprintf("TWSE %s: %s", dateStr, err.Error()))
			}
		} else {
			if len(twseRecs) > 0 {
				if err := r.saveRecords(job.ID, twseRecs); err != nil {
					log.Printf("[institutional] save TWSE %s failed: %v", dateStr, err)
					fail++
				} else {
					success++
					log.Printf("[institutional] TWSE %s saved %d records", dateStr, len(twseRecs))
				}
			} else {
				// 非交易日或無資料 → 跳過，仍計成功
				success++
				log.Printf("[institutional] TWSE %s: no data (holiday?)", dateStr)
			}
		}

		if r.requestDelay > 0 {
			time.Sleep(r.requestDelay)
		}

		tpexRecs, err := scraper.FetchTPEXInstitutional(date)
		if err != nil {
			fail++
			log.Printf("[institutional] TPEX %s failed: %v", dateStr, err)
			if len(failSamples) < 10 {
				failSamples = append(failSamples, fmt.Sprintf("TPEX %s: %s", dateStr, err.Error()))
			}
		} else {
			if len(tpexRecs) > 0 {
				if err := r.saveRecords(job.ID, tpexRecs); err != nil {
					log.Printf("[institutional] save TPEX %s failed: %v", dateStr, err)
					fail++
				} else {
					success++
					log.Printf("[institutional] TPEX %s saved %d records", dateStr, len(tpexRecs))
				}
			} else {
				success++
				log.Printf("[institutional] TPEX %s: no data (holiday?)", dateStr)
			}
		}

		// 更新進度
		if (i+1)%5 == 0 || i+1 == len(dates) {
			msg := fmt.Sprintf("處理中 %d/%d 日", i+1, len(dates))
			if err := r.db.Model(&models.InstitutionalSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{"success": success, "fail": fail, "message": msg}).Error; err != nil {
				log.Printf("[institutional] update progress failed: %v", err)
			}
		}

		if r.requestDelay > 0 && i < len(dates)-1 {
			time.Sleep(r.requestDelay)
		}
	}

	completedAt := time.Now()
	status := "completed"
	if success == 0 && fail > 0 {
		status = "failed"
	}

	message := fmt.Sprintf("完成：成功 %d，失敗 %d（共 %d 日）", success, fail, len(dates))
	if len(failSamples) > 0 {
		message += "\n\n失敗明細：\n"
		for _, s := range failSamples {
			message += "• " + s + "\n"
		}
	}

	if err := r.db.Model(&models.InstitutionalSyncJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       status,
			"completed_at": &completedAt,
			"success":      success,
			"fail":         fail,
			"message":      message,
		}).Error; err != nil {
		log.Printf("[institutional] finalize job failed: %v", err)
	}

	log.Printf("[institutional] job %d finished status=%s success=%d fail=%d dates=%d",
		job.ID, status, success, fail, len(dates))
}

func (r *Runner) saveRecords(jobID uint, records []scraper.InstitutionalRecord) error {
	if len(records) == 0 {
		return nil
	}

	rows := make([]models.InstitutionalTrading, 0, len(records))
	for _, rec := range records {
		rows = append(rows, models.InstitutionalTrading{
			JobID:       jobID,
			Symbol:      rec.Symbol,
			Date:        rec.Date,
			Market:      rec.Market,
			ForeignBuy:  rec.ForeignBuy,
			ForeignSell: rec.ForeignSell,
			ForeignNet:  rec.ForeignNet,
			TrustBuy:    rec.TrustBuy,
			TrustSell:   rec.TrustSell,
			TrustNet:    rec.TrustNet,
			DealerNet:   rec.DealerNet,
			TotalNet:    rec.TotalNet,
		})
	}

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "symbol"},
			{Name: "date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"job_id", "market",
			"foreign_buy", "foreign_sell", "foreign_net",
			"trust_buy", "trust_sell", "trust_net",
			"dealer_net", "total_net",
		}),
	}).Create(&rows).Error
}

// ─── 工具函式 ────────────────────────────────────────────────────────────────

func durationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func intEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// 未使用，保留供未來擴充
var _ = intEnv
