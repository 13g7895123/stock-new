package chips

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-backend/internal/models"

	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrJobRunning = errors.New("chips job already running")
var ErrNoSymbols = errors.New("沒有股票需要爬取（請先同步股票清單）")

var datePattern = regexp.MustCompile(`^\d{8}$`)

type Runner struct {
	db           *gorm.DB
	client       *http.Client
	concurrency  int
	requestDelay time.Duration

	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc
}

type scrapeResult struct {
	symbol  string
	success bool
	err     error
}

func NewRunner(db *gorm.DB) *Runner {
	return &Runner{
		db: db,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		concurrency:  intEnv("CHIPS_CONCURRENCY", 8),
		requestDelay: durationEnv("CHIPS_REQUEST_DELAY", 300*time.Millisecond),
	}
}

func (r *Runner) RecoverStaleJobs() error {
	return r.db.Model(&models.ChipsSyncJob{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": time.Now(),
			"message":      "backend restarted before job completed",
		}).Error
}

func (r *Runner) Trigger(symbol string) (int, error) {
	return r.TriggerWithScheme(symbol, "")
}

// TriggerWithScheme 由 handler 呼叫，帶入方案名稱記錄到 job 中
func (r *Runner) TriggerWithScheme(symbol, scheme string) (int, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return 0, ErrJobRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancelFn = cancel
	r.mu.Unlock()

	var symbols []string
	var err error
	if symbol != "" {
		symbols = []string{symbol}
	} else {
		symbols, err = r.loadSymbols()
	}
	if err != nil {
		r.clearRunning()
		return 0, err
	}
	if len(symbols) == 0 {
		r.clearRunning()
		return 0, ErrNoSymbols
	}

	go r.runJob(ctx, symbols, scheme)
	return len(symbols), nil
}

// Cancel 中止目前執行中的 job，回傳 false 表示沒有在跑。
func (r *Runner) Cancel() bool {
	r.mu.Lock()
	fn := r.cancelFn
	run := r.running
	r.mu.Unlock()
	if !run || fn == nil {
		return false
	}
	fn()
	return true
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) setRunning(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = v
}

func (r *Runner) clearRunning() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.cancelFn = nil
}

func (r *Runner) loadSymbols() ([]string, error) {
	var symbols []string
	cutoff := latestExpectedDataDate()
	if err := r.db.Raw(
		`SELECT s.symbol
		 FROM stocks s
		 LEFT JOIN (
		   SELECT symbol, MAX(data_date) AS latest_data_date
		   FROM chips_holder_snapshots
		   GROUP BY symbol
		 ) snapshots ON snapshots.symbol = s.symbol
		 WHERE s.symbol <> ''
		   AND (snapshots.latest_data_date IS NULL OR snapshots.latest_data_date < ?)
		 ORDER BY s.symbol ASC`,
		cutoff,
	).Scan(&symbols).Error; err != nil {
		return nil, err
	}
	return symbols, nil
}

func (r *Runner) runJob(ctx context.Context, symbols []string, scheme string) {
	defer r.clearRunning()

	job := models.ChipsSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Scheme:    scheme,
		Total:     len(symbols),
		Success:   0,
		Fail:      0,
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[chips] create job failed: %v", err)
		return
	}

	WriteLog(r.db, jobPtr(job.ID), "info", "job_start", "",
		fmt.Sprintf("開始爬取，共 %d 支股票", len(symbols)))

	workerCount := minInt(r.concurrency, len(symbols))
	jobs := make(chan string)
	results := make(chan scrapeResult, workerCount*2)

	// Dispatcher：依 ctx 控制是否繼續送 symbol
	go func() {
		defer close(jobs)
		for _, symbol := range symbols {
			select {
			case jobs <- symbol:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.worker(job.ID, ctx, jobs, results)
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	success := 0
	fail := 0
	processed := 0
	cancelled := false
	const maxFailureSamples = 20
	failureSamples := make([]string, 0, maxFailureSamples)

	for result := range results {
		processed++
		if result.success {
			success++
		} else {
			fail++
			errMsg := "unknown error"
			if result.err != nil {
				errMsg = result.err.Error()
			}
			log.Printf("[chips][%s] %v", result.symbol, errMsg)
			if len(failureSamples) < maxFailureSamples {
				failureSamples = append(failureSamples, fmt.Sprintf("%s: %s", result.symbol, errMsg))
			}
		}

		// 檢查是否已被取消
		select {
		case <-ctx.Done():
			cancelled = true
		default:
		}

		if processed%5 == 0 {
			message := fmt.Sprintf("處理中 %d/%d：%s", processed, len(symbols), result.symbol)
			if err := r.db.Model(&models.ChipsSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{"success": success, "fail": fail, "message": message}).Error; err != nil {
				log.Printf("[chips] update progress failed: %v", err)
			}
		}
	}

	completedAt := time.Now()
	status := "completed"
	if cancelled {
		status = "cancelled"
	} else if success == 0 && fail > 0 {
		status = "failed"
	}

	// 組合最終 message：成功/失敗統計 + 失敗範例
	message := fmt.Sprintf("完成：成功 %d，失敗 %d", success, fail)
	if cancelled {
		message = fmt.Sprintf("已取消（已處理 %d/%d，成功 %d，失敗 %d）", processed, len(symbols), success, fail)
	} else if len(failureSamples) > 0 {
		message += "\n\n失敗範例（前" + fmt.Sprintf("%d", len(failureSamples)) + "支）：\n"
		for _, s := range failureSamples {
			message += "• " + s + "\n"
		}
		if fail > len(failureSamples) {
			message += fmt.Sprintf("…（共 %d 支失敗，常見原因：ETF/權證/特別股不在來源網站追蹤範圍內）", fail)
		}
	}

	if err := r.db.Model(&models.ChipsSyncJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       status,
			"completed_at": &completedAt,
			"success":      success,
			"fail":         fail,
			"message":      message,
		}).Error; err != nil {
		log.Printf("[chips] finalize job failed: %v", err)
	}

	WriteLog(r.db, jobPtr(job.ID), "info", "job_end", "",
		fmt.Sprintf("完成 status=%s success=%d fail=%d", status, success, fail))
	log.Printf("[chips] job %d finished status=%s success=%d fail=%d", job.ID, status, success, fail)
}

func (r *Runner) worker(jobID uint, ctx context.Context, jobs <-chan string, results chan<- scrapeResult) {
	for symbol := range jobs {
		ok, err := r.scrapeSymbol(ctx, jobID, symbol)
		results <- scrapeResult{symbol: symbol, success: ok, err: err}
		if r.requestDelay > 0 {
			select {
			case <-time.After(r.requestDelay):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (r *Runner) scrapeSymbol(ctx context.Context, jobID uint, symbol string) (bool, error) {
	html, err := r.fetchPage(ctx, jobID, symbol)
	if err != nil {
		WriteLog(r.db, jobPtr(jobID), "error", "fetch_fail", symbol, err.Error())
		return false, err
	}

	dataDate, distributions, err := parseLatestSnapshot(html)
	if err != nil {
		WriteLog(r.db, jobPtr(jobID), "error", "parse_fail", symbol, err.Error())
		return false, err
	}
	if len(distributions) == 0 {
		WriteLog(r.db, jobPtr(jobID), "error", "parse_fail", symbol, "無可用分布資料（distributions 為空）")
		return false, errors.New("無可用分布資料")
	}

	if err := r.saveSnapshot(jobID, symbol, dataDate, distributions); err != nil {
		WriteLog(r.db, jobPtr(jobID), "error", "save_fail", symbol, err.Error())
		return false, err
	}
	return true, nil
}

func (r *Runner) fetchPage(ctx context.Context, jobID uint, symbol string) (string, error) {
	const maxRetries = 3
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// ctx 可中斷的 retry delay
		if attempt > 0 {
			select {
			case <-time.After(delays[attempt-1]):
			case <-ctx.Done():
				return "", context.Canceled
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://norway.twsthr.info/StockHolders.aspx?stock="+symbol, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Referer", "https://norway.twsthr.info/")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			resp.Body.Close()
			if err != nil {
				return "", err
			}
			html, err := doc.Html()
			if err != nil {
				return "", err
			}
			return html, nil
		}

		status := resp.StatusCode
		resp.Body.Close()

		// 403/429/503 可能是暫時限速，重試
		if status == http.StatusForbidden || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			lastErr = fmt.Errorf("unexpected status: %d", status)
			log.Printf("[chips][%s] HTTP %d，第 %d 次重試", symbol, status, attempt+1)
			WriteLog(r.db, jobPtr(jobID), "warn", "fetch_retry", symbol,
				fmt.Sprintf("HTTP %d，第 %d 次重試", status, attempt+1))
			continue
		}

		return "", fmt.Errorf("unexpected status: %d", status)
	}
	return "", lastErr
}

func parseLatestSnapshot(html string) (time.Time, []models.ChipsHolderDistribution, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return time.Time{}, nil, err
	}

	table := doc.Find("#details").First()
	if table.Length() == 0 {
		return time.Time{}, nil, errors.New("details table not found")
	}

	dataDate, err := extractLatestDate(table)
	if err != nil {
		return time.Time{}, nil, err
	}

	distributions := make([]models.ChipsHolderDistribution, 0, 16)
	tierRank := 0
	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := make([]string, 0, 8)
		row.Find("td").Each(func(_ int, td *goquery.Selection) {
			text := normalizeText(td.Text())
			cells = append(cells, text)
		})

		if len(cells) < 5 {
			return
		}

		label := normalizeRangeLabel(cells[1])
		if label == "" || strings.HasPrefix(label, "*") || label == "合計" {
			return
		}

		holderCount, ok1 := parseIntPtr(cells[2])
		shareCount, ok2 := parseInt64Ptr(cells[3])
		sharePct, ok3 := parseFloatPtr(cells[4])
		if !ok1 && !ok2 && !ok3 {
			return
		}

		tierRank++
		distributions = append(distributions, models.ChipsHolderDistribution{
			TierRank:    tierRank,
			RangeLabel:  label,
			HolderCount: holderCount,
			ShareCount:  shareCount,
			SharePct:    sharePct,
		})
	})

	if len(distributions) == 0 {
		return time.Time{}, nil, errors.New("no distributions parsed")
	}

	return dataDate, distributions, nil
}

func extractLatestDate(table *goquery.Selection) (time.Time, error) {
	var raw string
	table.Find("thead tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		row.Find("th, td").EachWithBreak(func(_ int, cell *goquery.Selection) bool {
			text := normalizeText(cell.Text())
			if datePattern.MatchString(text) {
				raw = text
				return false
			}
			return true
		})
		return raw == ""
	})
	if raw == "" {
		return time.Time{}, errors.New("latest data date not found")
	}
	t, err := time.ParseInLocation("20060102", raw, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (r *Runner) saveSnapshot(jobID uint, symbol string, dataDate time.Time, distributions []models.ChipsHolderDistribution) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		snapshot := models.ChipsHolderSnapshot{
			JobID:     jobID,
			Symbol:    symbol,
			DataDate:  dataDate,
			ScrapedAt: time.Now(),
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "symbol"}, {Name: "data_date"}},
			DoUpdates: clause.Assignments(map[string]any{
				"job_id":     jobID,
				"scraped_at": snapshot.ScrapedAt,
			}),
		}).Create(&snapshot).Error; err != nil {
			return err
		}

		if err := tx.Where("symbol = ? AND data_date = ?", symbol, dataDate).First(&snapshot).Error; err != nil {
			return err
		}

		if err := tx.Where("snapshot_id = ?", snapshot.ID).Delete(&models.ChipsHolderDistribution{}).Error; err != nil {
			return err
		}

		for i := range distributions {
			distributions[i].SnapshotID = snapshot.ID
		}

		if len(distributions) > 0 {
			if err := tx.CreateInBatches(distributions, 50).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\t", "")
	return strings.TrimSpace(value)
}

func normalizeRangeLabel(label string) string {
	label = normalizeText(label)
	label = strings.ReplaceAll(label, "~", "-")
	if label == "" || label == "持股張數分級" {
		return ""
	}
	if strings.Contains(label, "-") && !strings.Contains(label, "張") && !strings.Contains(label, "股") {
		return label + "張"
	}
	return label
}

func parseIntPtr(value string) (*int, bool) {
	v, err := strconv.Atoi(strings.ReplaceAll(value, ",", ""))
	if err != nil {
		return nil, false
	}
	return &v, true
}

func parseInt64Ptr(value string) (*int64, bool) {
	v, err := strconv.ParseInt(strings.ReplaceAll(value, ",", ""), 10, 64)
	if err != nil {
		return nil, false
	}
	return &v, true
}

func parseFloatPtr(value string) (*float64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(value, ",", ""), "%", ""), 64)
	if err != nil {
		return nil, false
	}
	return &v, true
}

func intEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		seconds, err := strconv.ParseFloat(value, 64)
		if err == nil && seconds >= 0 {
			return time.Duration(seconds * float64(time.Second))
		}
	}
	return fallback
}

func latestExpectedDataDate() time.Time {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	daysSinceFriday := (int(now.Weekday()) - int(time.Friday) + 7) % 7
	date := now.AddDate(0, 0, -daysSinceFriday)
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
