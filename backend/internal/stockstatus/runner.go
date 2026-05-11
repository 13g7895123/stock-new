package stockstatus

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stock-backend/internal/models"
	"stock-backend/internal/scraper"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrJobRunning = errors.New("stock status sync job already running")

type SyncResult struct {
	Total              int `json:"total"`
	Disposition        int `json:"disposition"`
	Attention          int `json:"attention"`
	DayTradeRestricted int `json:"day_trade_restricted"`
}

type Runner struct {
	db *gorm.DB

	mu      sync.Mutex
	running bool
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{db: db}
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.StockStatusSyncJob{}).
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

func (r *Runner) Trigger() (*SyncResult, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil, ErrJobRunning
	}
	r.running = true
	r.mu.Unlock()
	defer r.setRunning(false)

	return r.runJob()
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

func (r *Runner) runJob() (*SyncResult, error) {
	now := time.Now()
	job := models.StockStatusSyncJob{
		StartedAt: now,
		Status:    "running",
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		return nil, err
	}

	finish := func(status, message string, result *SyncResult, err error) (*SyncResult, error) {
		completedAt := time.Now()
		total := 0
		if result != nil {
			total = result.Total
		}
		if updateErr := r.db.Model(&models.StockStatusSyncJob{}).
			Where("id = ?", job.ID).
			Updates(map[string]any{
				"status":       status,
				"completed_at": &completedAt,
				"total":        total,
				"message":      message,
			}).Error; updateErr != nil {
			log.Printf("[stock-status-sync] update job failed: %v", updateErr)
		}
		return result, err
	}

	records, err := scraper.FetchOfficialStockStatuses(now)
	if err != nil {
		return finish("failed", err.Error(), nil, err)
	}

	statusByKey := make(map[string]models.StockStatus, len(records))
	result := &SyncResult{}
	for _, record := range records {
		status := models.StockStatus{
			Symbol:     record.Symbol,
			Name:       record.Name,
			Market:     record.Market,
			Type:       record.Type,
			SourceDate: record.SourceDate,
			StartDate:  record.StartDate,
			EndDate:    record.EndDate,
			Reason:     record.Reason,
			Measure:    record.Measure,
			Detail:     record.Detail,
			RawPeriod:  record.RawPeriod,
			SourceURL:  record.SourceURL,
			FetchedAt:  record.FetchedAt,
		}
		key := statusIdentityKey(status)
		if existing, ok := statusByKey[key]; ok {
			existing.Reason = mergeText(existing.Reason, status.Reason)
			existing.Measure = mergeText(existing.Measure, status.Measure)
			existing.Detail = mergeText(existing.Detail, status.Detail)
			existing.RawPeriod = mergeText(existing.RawPeriod, status.RawPeriod)
			existing.SourceURL = mergeText(existing.SourceURL, status.SourceURL)
			if status.FetchedAt.After(existing.FetchedAt) {
				existing.FetchedAt = status.FetchedAt
			}
			statusByKey[key] = existing
			continue
		}
		statusByKey[key] = status
		switch record.Type {
		case models.StockStatusDisposition:
			result.Disposition++
		case models.StockStatusAttention:
			result.Attention++
		case models.StockStatusDayTradeRestricted:
			result.DayTradeRestricted++
		}
	}

	statuses := make([]models.StockStatus, 0, len(statusByKey))
	for _, status := range statusByKey {
		statuses = append(statuses, status)
	}
	result.Total = len(statuses)

	if len(statuses) > 0 {
		if err := r.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "symbol"},
				{Name: "market"},
				{Name: "type"},
				{Name: "start_date"},
				{Name: "end_date"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"name",
				"source_date",
				"reason",
				"measure",
				"detail",
				"raw_period",
				"source_url",
				"fetched_at",
				"updated_at",
			}),
		}).CreateInBatches(&statuses, 500).Error; err != nil {
			return finish("failed", err.Error(), result, err)
		}
	}

	message := fmt.Sprintf("完成：處置 %d、注意 %d、限當沖 %d", result.Disposition, result.Attention, result.DayTradeRestricted)
	return finish("completed", message, result, nil)
}

func statusIdentityKey(status models.StockStatus) string {
	return strings.Join([]string{
		status.Symbol,
		status.Market,
		status.Type,
		status.StartDate.Format("2006-01-02"),
		status.EndDate.Format("2006-01-02"),
	}, "|")
}

func mergeText(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" || strings.Contains(a, b) {
		return a
	}
	return a + "\n" + b
}
