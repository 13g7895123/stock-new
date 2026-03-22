package handlers

import (
	"net/http"
	"strconv"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PriceHandler struct {
	db *gorm.DB
}

func NewPriceHandler(db *gorm.DB) *PriceHandler {
	return &PriceHandler{db: db}
}

// List  GET /api/stocks/:symbol/prices?from=2024-01-01&to=2024-12-31&limit=500
func (h *PriceHandler) List(c *gin.Context) {
	symbol := c.Param("symbol")

	q := h.db.Model(&models.DailyPrice{}).
		Where("symbol = ?", symbol).
		Order("date ASC")

	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("date >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("date <= ?", t)
		}
	}

	limit := 500
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}

	var prices []models.DailyPrice
	if err := q.Limit(limit).Find(&prices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prices)
}

// Latest  GET /api/stocks/:symbol/prices/latest
func (h *PriceHandler) Latest(c *gin.Context) {
	symbol := c.Param("symbol")
	var price models.DailyPrice
	if err := h.db.Where("symbol = ?", symbol).Order("date DESC").First(&price).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no price data"})
		return
	}
	c.JSON(http.StatusOK, price)
}
