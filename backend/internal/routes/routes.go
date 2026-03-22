package routes

import (
	"net/http"

	"stock-backend/internal/handlers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Setup(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	stockHandler := handlers.NewStockHandler(db)
	scraperHandler := handlers.NewScraperHandler(db)

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

		scraperGroup := api.Group("/scraper")
		{
			scraperGroup.GET("/stocks", scraperHandler.SyncStocksSSE)
		}
	}

	return r
}
