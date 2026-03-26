package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ─── 靜態功能目錄 ─────────────────────────────────────────────────────────────
// 目錄由後端定義（唯讀），使用者只能修改每個功能的 config。

type SchemeInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	NeedService string `json:"need_service,omitempty"` // 需要外部服務 e.g. "python_scraper"
}

type FeatureInfo struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Description string       `json:"description"`
	Category    string       `json:"category"` // "scraper" | "sync"
	Schemes     []SchemeInfo `json:"schemes"`
}

// FeatureConfig 是儲存在 DB 的用戶可設定部分
type FeatureConfig struct {
	Primary         string `json:"primary"`
	FallbackEnabled bool   `json:"fallback_enabled"`
	Fallback        string `json:"fallback"`
	FallbackTrigger string `json:"fallback_trigger"` // "error" | "empty_data"
}

var featureCatalog = []FeatureInfo{
	{
		ID:          "chips_pyramid",
		Label:       "籌碼金字塔",
		Description: "抓取 norway.twsthr.info 各股持股分佈資料，每週更新",
		Category:    "scraper",
		Schemes: []SchemeInfo{
			{
				ID:          "go_http",
				Label:       "Go HTTP（內建）",
				Description: "Go 後端直接發 HTTP 請求爬取，速度快，無需額外服務",
			},
			{
				ID:          "python_http",
				Label:       "Python aiohttp（輕量）",
				Description: "Python asyncio + aiohttp 爬取，不需要瀏覽器，較節省資源",
				NeedService: "python_scraper",
			},
			{
				ID:          "python_playwright",
				Label:       "Python Playwright（瀏覽器）",
				Description: "無頭 Chromium 瀏覽器模擬，相容性最高，資源消耗較大",
				NeedService: "python_scraper",
			},
		},
	},
	{
		ID:          "stock_list",
		Label:       "股票清單同步",
		Description: "同步 TWSE 上市、TPEx 上櫃股票清單至本地資料庫",
		Category:    "sync",
		Schemes: []SchemeInfo{
			{
				ID:          "twse_tpex_api",
				Label:       "TWSE + TPEx OpenAPI",
				Description: "直接呼叫交易所官方 OpenAPI，資料最權威",
			},
		},
	},
	{
		ID:          "daily_price",
		Label:       "每日日K",
		Description: "爬取所有股票的日K OHLCV 歷史資料",
		Category:    "scraper",
		Schemes: []SchemeInfo{
			{
				ID:          "twse_tpex_api",
				Label:       "TWSE + TPEx API（主要）",
				Description: "直接呼叫交易所 API，為最原始的官方資料來源",
			},
			{
				ID:          "broker_api",
				Label:       "券商 API（備援）",
				Description: "透過多家券商聚合 API 取得資料，可作為備案降低爬取失敗率",
			},
		},
	},
}

// 預設 config（當 DB 尚無設定時使用）
var defaultConfigs = map[string]FeatureConfig{
	"chips_pyramid": {
		Primary:         "go_http",
		FallbackEnabled: false,
		Fallback:        "python_http",
		FallbackTrigger: "error",
	},
	"stock_list": {
		Primary:         "twse_tpex_api",
		FallbackEnabled: false,
		Fallback:        "",
		FallbackTrigger: "error",
	},
	"daily_price": {
		Primary:         "twse_tpex_api",
		FallbackEnabled: true,
		Fallback:        "broker_api",
		FallbackTrigger: "error",
	},
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type SettingsHandler struct {
	db *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) settingKey(featureID string) string {
	return "feature_config." + featureID
}

func (h *SettingsHandler) loadConfig(featureID string) FeatureConfig {
	def := defaultConfigs[featureID]
	var s models.AppSetting
	if err := h.db.First(&s, "key = ?", h.settingKey(featureID)).Error; err != nil {
		return def
	}
	var cfg FeatureConfig
	if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
		return def
	}
	return cfg
}

type FeatureResponse struct {
	FeatureInfo
	Config FeatureConfig `json:"config"`
}

// GetAll GET /api/settings/features
// 回傳所有功能的目錄資訊 + 當前設定
func (h *SettingsHandler) GetAll(c *gin.Context) {
	result := make([]FeatureResponse, 0, len(featureCatalog))
	for _, f := range featureCatalog {
		result = append(result, FeatureResponse{
			FeatureInfo: f,
			Config:      h.loadConfig(f.ID),
		})
	}
	c.JSON(http.StatusOK, result)
}

// UpdateFeature PUT /api/settings/features/:id
// 更新指定功能的設定
func (h *SettingsHandler) UpdateFeature(c *gin.Context) {
	featureID := c.Param("id")

	// 確認 feature 存在
	found := false
	for _, f := range featureCatalog {
		if f.ID == featureID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "feature not found"})
		return
	}

	var cfg FeatureConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "格式錯誤: " + err.Error()})
		return
	}

	// 驗證 primary scheme 合法
	validPrimary := false
	var validSchemeIDs []string
	for _, f := range featureCatalog {
		if f.ID == featureID {
			for _, s := range f.Schemes {
				validSchemeIDs = append(validSchemeIDs, s.ID)
				if s.ID == cfg.Primary {
					validPrimary = true
				}
			}
		}
	}
	if !validPrimary {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "無效的 primary scheme",
			"valid_schemes": validSchemeIDs,
		})
		return
	}

	valueBytes, _ := json.Marshal(cfg)
	setting := models.AppSetting{
		Key:       h.settingKey(featureID),
		Value:     string(valueBytes),
		UpdatedAt: time.Now(),
	}

	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, FeatureResponse{
		FeatureInfo: func() FeatureInfo {
			for _, f := range featureCatalog {
				if f.ID == featureID {
					return f
				}
			}
			return FeatureInfo{}
		}(),
		Config: cfg,
	})
}

// GetFeatureConfig 供其他 handler 讀取指定功能設定的 helper
func GetFeatureConfig(db *gorm.DB, featureID string) FeatureConfig {
	h := &SettingsHandler{db: db}
	return h.loadConfig(featureID)
}
