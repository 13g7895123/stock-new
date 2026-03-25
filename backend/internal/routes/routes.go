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
	priceHandler := handlers.NewPriceHandler(db)
	chipsHandler := handlers.NewChipsHandler(db)
	tagHandler := handlers.NewTagHandler(db)
	priceSyncHandler := handlers.NewPriceSyncHandler(db)
	debugHandler := handlers.NewDebugHandler(db)

	api := r.Group("/api")
	{
		stocks := api.Group("/stocks")
		{
			stocks.GET("", tagHandler.ListStocks) // 覆寫：支援 industry/tag_id/q 篩選 + Preload Tags
			stocks.GET("/:symbol", stockHandler.GetBySymbol)
			stocks.POST("", stockHandler.Create)
			stocks.PUT("/:symbol", stockHandler.Update)
			stocks.DELETE("/:symbol", stockHandler.Delete)
			// 日K 價量
			stocks.GET("/:symbol/prices", priceHandler.List)
			stocks.GET("/:symbol/prices/latest", priceHandler.Latest)
			// Tags 指派
			stocks.PUT("/:symbol/tags", tagHandler.SetStockTags)
		}

		// 產業列表
		api.GET("/industries", tagHandler.ListIndustries)

		// Tags CRUD
		tags := api.Group("/tags")
		{
			tags.GET("", tagHandler.List)
			tags.POST("", tagHandler.Create)
			tags.PUT("/:id", tagHandler.Update)
			tags.DELETE("/:id", tagHandler.Delete)
		}

		scraperGroup := api.Group("/scraper")
		{
			scraperGroup.GET("/stocks", scraperHandler.SyncStocksSSE)
			scraperGroup.GET("/prices", scraperHandler.SyncPricesSSE)
			scraperGroup.GET("/prices/stock/:symbol", scraperHandler.RefreshStockSSE)
			// 全股票歷史日K批次爬取
			scraperGroup.GET("/prices/all/status", priceSyncHandler.Status)
			scraperGroup.POST("/prices/all/trigger", priceSyncHandler.Trigger)
			scraperGroup.POST("/prices/all/test", priceSyncHandler.TestSingle)
		}

		chips := api.Group("/chips")
		{
			chips.GET("/status", chipsHandler.Status)
			chips.POST("/trigger", chipsHandler.Trigger)
			chips.POST("/trigger-single", chipsHandler.TriggerSingle)
		}

		debugGroup := api.Group("/debug")
		{
			debugGroup.GET("/raw-month", debugHandler.RawMonth)
		}
	}

	return r
}
