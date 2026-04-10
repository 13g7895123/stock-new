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
	majorHandler := handlers.NewMajorHandler(db)
	institutionalHandler := handlers.NewInstitutionalHandler(db)
	groupHandler := handlers.NewGroupHandler(db)
	winrateHandler := handlers.NewWinrateHandler(db)
	technicalHandler := handlers.NewTechnicalHandler(db)

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
			// Groups 指派
			stocks.PUT("/:symbol/groups", groupHandler.SetStockGroups)
			// 券商勝率
			stocks.GET("/:symbol/broker-winrate", winrateHandler.GetWinrateBySymbol)
			stocks.GET("/:symbol/broker-winrate/events", winrateHandler.GetEventsByBroker)
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

		// Groups CRUD
		groupsRoute := api.Group("/groups")
		{
			groupsRoute.GET("", groupHandler.List)
			groupsRoute.POST("", groupHandler.Create)
			groupsRoute.PUT("/:id", groupHandler.Update)
			groupsRoute.DELETE("/:id", groupHandler.Delete)
		}

		// 券商勝率
		winrateRoute := api.Group("/winrate")
		{
			winrateRoute.GET("/status", winrateHandler.Status)
			winrateRoute.POST("/trigger", winrateHandler.Trigger)
			winrateRoute.POST("/trigger-single", winrateHandler.TriggerSingle)
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
			chips.GET("/logs", chipsHandler.GetLogs)
			chips.POST("/trigger", chipsHandler.Trigger)
			chips.POST("/trigger-single", chipsHandler.TriggerSingle)
			chips.POST("/cancel", chipsHandler.Cancel)
			chips.GET("/:symbol/latest", chipsHandler.GetSymbolLatest)
		}

		major := api.Group("/major")
		{
			major.GET("/status", majorHandler.Status)
			major.POST("/trigger", majorHandler.Trigger)
			major.POST("/trigger-single", majorHandler.TriggerSingle)
			major.POST("/test", majorHandler.TestSingle)
			major.GET("/:symbol", majorHandler.GetBySymbol)
		}

		institutional := api.Group("/institutional")
		{
			institutional.GET("/status", institutionalHandler.Status)
			institutional.POST("/trigger", institutionalHandler.Trigger)
			institutional.GET("/:symbol", institutionalHandler.GetBySymbol)
		}

		debugGroup := api.Group("/debug")
		{
			debugGroup.GET("/raw-month", debugHandler.RawMonth)
			debugGroup.GET("/broker-fetch", debugHandler.BrokerFetch)
		}

		technicalGroup := api.Group("/technical")
		{
			technicalGroup.GET("/screener", technicalHandler.Screener)
		}

		realtimeHandler := handlers.NewRealtimeHandler(db)
		api.GET("/realtime/:symbol", realtimeHandler.Quote)

		dbViewerHandler := handlers.NewDBViewerHandler(db)
		adminGroup := api.Group("/admin/db")
		{
			adminGroup.GET("/tables", dbViewerHandler.ListTables)
			adminGroup.GET("/tables/:name/columns", dbViewerHandler.TableColumns)
			adminGroup.GET("/tables/:name/data", dbViewerHandler.TableData)
		}

		settingsHandler := handlers.NewSettingsHandler(db)
		settingsGroup := api.Group("/settings")
		{
			settingsGroup.GET("/features", settingsHandler.GetAll)
			settingsGroup.PUT("/features/:id", settingsHandler.UpdateFeature)
		}

		scheduleHandler := handlers.NewScheduleHandler(db)
		scheduleGroup := api.Group("/schedules")
		{
			scheduleGroup.GET("/holidays", scheduleHandler.GetHolidays)
			scheduleGroup.PUT("/holidays", scheduleHandler.SetHolidays)
			scheduleGroup.GET("", scheduleHandler.GetAll)
			scheduleGroup.PUT("/:task_id", scheduleHandler.Update)
			scheduleGroup.POST("/:task_id/run", scheduleHandler.ManualRun)
		}
	}

	return r
}
