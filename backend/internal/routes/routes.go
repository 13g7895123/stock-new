package routes

import (
	"net/http"

	"stock-backend/internal/handlers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	stockHandler := handlers.NewStockHandler(db)

	api := r.Group("/api")
	{
		stocks := api.Group("/stocks")
		{
			stocks.GET("", stockHandler.List)
			stocks.GET("/:id", stockHandler.Get)
			stocks.POST("", stockHandler.Create)
			stocks.PUT("/:id", stockHandler.Update)
			stocks.DELETE("/:id", stockHandler.Delete)
		}
	}

	return r
}
