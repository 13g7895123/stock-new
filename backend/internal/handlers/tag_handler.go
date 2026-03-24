package handlers

import (
	"net/http"
	"strconv"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TagHandler struct {
	db *gorm.DB
}

func NewTagHandler(db *gorm.DB) *TagHandler {
	return &TagHandler{db: db}
}

// GET /api/tags
func (h *TagHandler) List(c *gin.Context) {
	var tags []models.Tag
	if err := h.db.Order("name asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// POST /api/tags  body: {"name":"科技","color":"#3b82f6"}
func (h *TagHandler) Create(c *gin.Context) {
	var tag models.Tag
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if tag.Color == "" {
		tag.Color = "#6b7280"
	}
	if err := h.db.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tag)
}

// PUT /api/tags/:id  body: {"name":"...","color":"..."}
func (h *TagHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := h.db.First(&tag, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != "" {
		tag.Name = body.Name
	}
	if body.Color != "" {
		tag.Color = body.Color
	}
	h.db.Save(&tag)
	c.JSON(http.StatusOK, tag)
}

// DELETE /api/tags/:id
func (h *TagHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Delete(&models.Tag{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// PUT /api/stocks/:symbol/tags   body: {"tag_ids":[1,2,3]}
// 全量覆寫該股票的 tags
func (h *TagHandler) SetStockTags(c *gin.Context) {
	symbol := c.Param("symbol")
	var stock models.Stock
	if err := h.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stock not found"})
		return
	}

	var body struct {
		TagIDs []uint `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tags []models.Tag
	if len(body.TagIDs) > 0 {
		if err := h.db.Find(&tags, body.TagIDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.db.Model(&stock).Association("Tags").Replace(tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "tag_count": len(tags)})
}

// GET /api/stocks  — 覆寫 list 支援 industry / tag 篩選 + load tags
// 此方法掛在 TagHandler 避免循環依賴，供 routes 選用

// GET /api/stocks?industry=半導體&tag_id=1&q=台積電
func (h *TagHandler) ListStocks(c *gin.Context) {
	q := c.Query("q")
	industry := c.Query("industry")
	tagIDStr := c.Query("tag_id")

	db := h.db.Preload("Tags")

	if q != "" {
		like := "%" + q + "%"
		db = db.Where("symbol ILIKE ? OR name ILIKE ?", like, like)
	}
	if industry != "" {
		db = db.Where("industry = ?", industry)
	}
	if tagIDStr != "" {
		tagID, err := strconv.Atoi(tagIDStr)
		if err == nil {
			db = db.Joins("JOIN stock_tags ON stock_tags.stock_id = stocks.id").
				Where("stock_tags.tag_id = ?", tagID)
		}
	}

	var stocks []models.Stock
	if err := db.Find(&stocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stocks)
}

// GET /api/industries  — 回傳不重複的產業類別列表
func (h *TagHandler) ListIndustries(c *gin.Context) {
	var industries []string
	if err := h.db.Model(&models.Stock{}).
		Where("industry != ''").
		Distinct().
		Pluck("industry", &industries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, industries)
}
