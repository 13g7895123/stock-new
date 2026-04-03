package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ─── 任務目錄（唯讀，後端定義）─────────────────────────────────────────────────

type ScheduleTaskMeta struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	HasParams   bool   `json:"has_params"` // 是否有額外參數（如 days）
}

var scheduleTaskCatalog = []ScheduleTaskMeta{
	{
		ID:          "chips_pyramid",
		Label:       "籌碼金字塔",
		Description: "抓取 norway.twsthr.info 各股持股分佈資料",
		HasParams:   false,
	},
	{
		ID:          "stock_list",
		Label:       "股票清單同步",
		Description: "同步 TWSE 上市、TPEx 上櫃股票清單至本地資料庫",
		HasParams:   false,
	},
	{
		ID:          "daily_price",
		Label:       "全量日K回填",
		Description: "爬取所有股票的完整歷史日K OHLCV 資料",
		HasParams:   false,
	},
	{
		ID:          "major_chips",
		Label:       "主力進出",
		Description: "爬取各券商分點的買賣超明細（可設定 days 參數）",
		HasParams:   true,
	},
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type ScheduleHandler struct {
	db *gorm.DB
}

func NewScheduleHandler(db *gorm.DB) *ScheduleHandler {
	return &ScheduleHandler{db: db}
}

type ScheduleResponse struct {
	ScheduleTaskMeta
	Schedule models.TaskSchedule `json:"schedule"`
}

// loadOrDefault 從 DB 讀取排程，若無記錄則回傳預設值
func (h *ScheduleHandler) loadOrDefault(taskID string) models.TaskSchedule {
	var s models.TaskSchedule
	if err := h.db.First(&s, "task_id = ?", taskID).Error; err != nil {
		return models.TaskSchedule{
			TaskID:  taskID,
			Enabled: false,
			Type:    "manual",
			Hour:    10,
			Minute:  0,
			Weekday: 0,
			Params:  "{}",
		}
	}
	return s
}

// GetAll GET /api/schedules
func (h *ScheduleHandler) GetAll(c *gin.Context) {
	result := make([]ScheduleResponse, 0, len(scheduleTaskCatalog))
	for _, meta := range scheduleTaskCatalog {
		result = append(result, ScheduleResponse{
			ScheduleTaskMeta: meta,
			Schedule:         h.loadOrDefault(meta.ID),
		})
	}
	c.JSON(http.StatusOK, result)
}

// Update PUT /api/schedules/:task_id
func (h *ScheduleHandler) Update(c *gin.Context) {
	taskID := c.Param("task_id")

	// 確認任務存在
	found := false
	for _, m := range scheduleTaskCatalog {
		if m.ID == taskID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var body struct {
		Enabled bool   `json:"enabled"`
		Type    string `json:"type"` // manual|daily|weekly
		Hour    int    `json:"hour"`
		Minute  int    `json:"minute"`
		Weekday int    `json:"weekday"`
		Params  string `json:"params"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "格式錯誤: " + err.Error()})
		return
	}

	// 驗證
	if body.Type != "manual" && body.Type != "daily" && body.Type != "weekly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必須為 manual / daily / weekly"})
		return
	}
	if body.Hour < 0 || body.Hour > 23 || body.Minute < 0 || body.Minute > 59 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hour 需為 0-23，minute 需為 0-59"})
		return
	}
	if body.Weekday < 0 || body.Weekday > 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weekday 需為 0-6（0=週日）"})
		return
	}
	if body.Params == "" {
		body.Params = "{}"
	}

	now := time.Now()
	nextRun := calcNextRun(body.Type, body.Hour, body.Minute, body.Weekday, now)

	s := models.TaskSchedule{
		TaskID:    taskID,
		Enabled:   body.Enabled,
		Type:      body.Type,
		Hour:      body.Hour,
		Minute:    body.Minute,
		Weekday:   body.Weekday,
		Params:    body.Params,
		NextRunAt: nextRun,
		UpdatedAt: now,
	}

	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "type", "hour", "minute", "weekday", "params", "next_run_at", "updated_at"}),
	}).Create(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, s)
}

// ManualRun POST /api/schedules/:task_id/run
// 立即執行一次任務（不走排程計時）
func (h *ScheduleHandler) ManualRun(c *gin.Context) {
	taskID := c.Param("task_id")

	// 讀取 params（major_chips 需要 days）
	s := h.loadOrDefault(taskID)
	if err := dispatchTask(h.db, taskID, s.Params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "task_id": taskID})
}

// ─── 任務派送 ──────────────────────────────────────────────────────────────────

func dispatchTask(db *gorm.DB, taskID, params string) error {
	switch taskID {
	case "chips_pyramid":
		TriggerCron(db)
		return nil
	case "stock_list":
		return SyncStocksCron(db)
	case "daily_price":
		return TriggerPriceCron(db)
	case "major_chips":
		var p struct {
			Days int `json:"days"`
		}
		_ = json.Unmarshal([]byte(params), &p)
		if p.Days <= 0 {
			p.Days = 1
		}
		return TriggerMajorCron(db, p.Days)
	}
	return fmt.Errorf("unknown task: %s", taskID)
}

// ─── 排程器（在 main.go 以 goroutine 啟動）──────────────────────────────────────

// RunScheduler 每分鐘檢查所有啟用的排程，到時即觸發對應任務。
// 使用 lastFired 防止同一分鐘內重複觸發。
func RunScheduler(db *gorm.DB) {
	loc, _ := time.LoadLocation("Asia/Taipei")
	lastFired := map[string]time.Time{}

	for {
		now := time.Now().In(loc)
		// 對齊到整分鐘後再執行（避免秒級漂移）
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		time.Sleep(time.Until(nextMinute))

		now = time.Now().In(loc)
		var schedules []models.TaskSchedule
		if err := db.Find(&schedules).Error; err != nil {
			log.Printf("[scheduler] 讀取排程失敗: %v", err)
			continue
		}

		for _, s := range schedules {
			if !s.Enabled || s.Type == "manual" {
				continue
			}
			if !shouldFire(s, now) {
				continue
			}
			// 防止同一分鐘重複觸發
			key := s.TaskID
			if t, ok := lastFired[key]; ok && now.Sub(t) < 90*time.Second {
				continue
			}
			lastFired[key] = now

			log.Printf("[scheduler] 觸發任務 %s（type=%s %02d:%02d weekday=%d）",
				s.TaskID, s.Type, s.Hour, s.Minute, s.Weekday)

			go func(taskID, params string) {
				if err := dispatchTask(db, taskID, params); err != nil {
					log.Printf("[scheduler] 任務 %s 執行失敗: %v", taskID, err)
				}
				// 更新 last_run_at + next_run_at
				n := time.Now()
				nextRun := calcNextRun(s.Type, s.Hour, s.Minute, s.Weekday, n)
				db.Model(&models.TaskSchedule{}).
					Where("task_id = ?", taskID).
					Updates(map[string]any{
						"last_run_at": n,
						"next_run_at": nextRun,
					})
			}(s.TaskID, s.Params)
		}
	}
}

// shouldFire 判斷排程是否應在 now 這個分鐘觸發
func shouldFire(s models.TaskSchedule, now time.Time) bool {
	if now.Hour() != s.Hour || now.Minute() != s.Minute {
		return false
	}
	if s.Type == "daily" {
		return true
	}
	if s.Type == "weekly" {
		return int(now.Weekday()) == s.Weekday
	}
	return false
}

// calcNextRun 計算下一次觸發時間（nil = 手動模式）
func calcNextRun(schedType string, hour, minute, weekday int, from time.Time) *time.Time {
	loc, _ := time.LoadLocation("Asia/Taipei")
	t := from.In(loc)

	if schedType == "manual" {
		return nil
	}

	// 候選時間：今天的 hour:minute
	candidate := time.Date(t.Year(), t.Month(), t.Day(), hour, minute, 0, 0, loc)

	if schedType == "daily" {
		// 如果今天的時間已過，則選明天
		if !candidate.After(t) {
			candidate = candidate.Add(24 * time.Hour)
		}
		return &candidate
	}

	if schedType == "weekly" {
		// 找到下一個符合星期的日期
		daysAhead := (weekday - int(t.Weekday()) + 7) % 7
		if daysAhead == 0 && !candidate.After(t) {
			daysAhead = 7
		}
		candidate = candidate.AddDate(0, 0, daysAhead)
		return &candidate
	}

	return nil
}
