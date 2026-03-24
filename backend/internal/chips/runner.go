package chips

import (
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

	mu      sync.Mutex
	running bool
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

func (r *Runner) runJob(symbols []string) {
	defer r.setRunning(false)

	job := models.ChipsSyncJob{
		StartedAt: time.Now(),
		Status:    "running",
		Total:     len(symbols),
		Success:   0,
		Fail:      0,
		Message:   "已啟動",
	}
	if err := r.db.Create(&job).Error; err != nil {
		log.Printf("[chips] create job failed: %v", err)
		return
	}

	jobs := make(chan string)
	results := make(chan scrapeResult, len(symbols))
	workerCount := minInt(r.concurrency, len(symbols))
	for i := 0; i < workerCount; i++ {
		go r.worker(job.ID, jobs, results)
	}

	go func() {
		defer close(jobs)
		for _, symbol := range symbols {
			jobs <- symbol
		}
	}()

	success := 0
	fail := 0
	for processed := 1; processed <= len(symbols); processed++ {
		result := <-results
		if result.success {
			success++
		} else {
			fail++
			if result.err != nil {
				log.Printf("[chips][%s] %v", result.symbol, result.err)
			}
		}

		if processed%5 == 0 || processed == len(symbols) {
			message := fmt.Sprintf("處理中 %d/%d：%s", processed, len(symbols), result.symbol)
			if err := r.db.Model(&models.ChipsSyncJob{}).
				Where("id = ?", job.ID).
				Updates(map[string]any{"success": success, "fail": fail, "message": message}).Error; err != nil {
				log.Printf("[chips] update progress failed: %v", err)
			}
		}
	}

	completedAt := time.Now()
	message := fmt.Sprintf("完成：成功 %d，失敗 %d", success, fail)
	status := "completed"
	if success == 0 && fail > 0 {
		status = "failed"
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

	log.Printf("[chips] job %d finished status=%s success=%d fail=%d", job.ID, status, success, fail)
}

func (r *Runner) worker(jobID uint, jobs <-chan string, results chan<- scrapeResult) {
	for symbol := range jobs {
		ok, err := r.scrapeSymbol(jobID, symbol)
		results <- scrapeResult{symbol: symbol, success: ok, err: err}
		if r.requestDelay > 0 {
			time.Sleep(r.requestDelay)
		}
	}
}

func (r *Runner) scrapeSymbol(jobID uint, symbol string) (bool, error) {
	html, err := r.fetchPage(symbol)
	if err != nil {
		return false, err
	}

	dataDate, distributions, err := parseLatestSnapshot(html)
	if err != nil {
		return false, err
	}
	if len(distributions) == 0 {
		return false, errors.New("無可用分布資料")
	}

	if err := r.saveSnapshot(jobID, symbol, dataDate, distributions); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Runner) fetchPage(symbol string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://norway.twsthr.info/StockHolders.aspx?stock="+symbol, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}
	html, err := doc.Html()
	if err != nil {
		return "", err
	}
	return html, nil
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