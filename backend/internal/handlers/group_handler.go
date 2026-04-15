package handlers

import (
	"net/http"
	"strconv"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GroupHandler struct {
	db *gorm.DB
}

func NewGroupHandler(db *gorm.DB) *GroupHandler {
	return &GroupHandler{db: db}
}

// GET /api/groups
func (h *GroupHandler) List(c *gin.Context) {
	var groups []models.StockGroup
	if err := h.db.Order("name asc").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// POST /api/groups  body: {"name":"...","description":"...","color":"#..."}
func (h *GroupHandler) Create(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	g := models.StockGroup{
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	}
	if g.Color == "" {
		g.Color = "#3b82f6"
	}
	if err := h.db.Create(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

// PUT /api/groups/:id  body: {"name":"...","description":"...","color":"#..."}
func (h *GroupHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var g models.StockGroup
	if err := h.db.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != "" {
		g.Name = body.Name
	}
	if body.Color != "" {
		g.Color = body.Color
	}
	g.Description = body.Description
	h.db.Save(&g)
	c.JSON(http.StatusOK, g)
}

// DELETE /api/groups/:id
func (h *GroupHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var g models.StockGroup
	if err := h.db.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	// 先清除 many2many 關聯，再刪除群組
	if err := h.db.Model(&g).Association("Stocks").Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Delete(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// PUT /api/stocks/:symbol/groups  body: {"group_ids":[1,2,3]}
// 全量覆寫該股票的 groups
func (h *GroupHandler) SetStockGroups(c *gin.Context) {
	symbol := c.Param("symbol")
	var stock models.Stock
	if err := h.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stock not found"})
		return
	}

	var body struct {
		GroupIDs []uint `json:"group_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var groups []models.StockGroup
	if len(body.GroupIDs) > 0 {
		if err := h.db.Find(&groups, body.GroupIDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := h.db.Model(&stock).Association("Groups").Replace(groups); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/groups/:id/members  body: {"symbols":["1101","2330"]}
// 批次將股票加入群組（append，不覆寫現有成員）
func (h *GroupHandler) AddMembers(c *gin.Context) {
	id := c.Param("id")
	var g models.StockGroup
	if err := h.db.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	var body struct {
		Symbols []string `json:"symbols"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols required"})
		return
	}
	var stocks []models.Stock
	if err := h.db.Where("symbol IN ?", body.Symbols).Find(&stocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&g).Association("Stocks").Append(stocks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "added": len(stocks)})
}

// DELETE /api/groups/:id/members  body: {"symbols":["1101","2330"]}
// 批次將股票從群組移除
func (h *GroupHandler) RemoveMembers(c *gin.Context) {
	id := c.Param("id")
	var g models.StockGroup
	if err := h.db.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	var body struct {
		Symbols []string `json:"symbols"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols required"})
		return
	}
	var stocks []models.Stock
	if err := h.db.Where("symbol IN ?", body.Symbols).Find(&stocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&g).Association("Stocks").Delete(stocks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "removed": len(stocks)})
}

// ListStocksByGroup 供 tag_handler.go 使用的 group_id Joins（內部輔助）
func GroupIDJoin(db *gorm.DB, groupIDStr string) *gorm.DB {
	id, err := strconv.Atoi(groupIDStr)
	if err != nil || id == 0 {
		return db
	}
	return db.Joins("JOIN stock_group_members ON stock_group_members.stock_id = stocks.id").
		Where("stock_group_members.stock_group_id = ?", id)
}
