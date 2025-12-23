package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cmengs/xray-react/collector/internal/collector"
	"github.com/cmengs/xray-react/collector/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 初始化存储（SQLite PoC）
	dbPath := os.Getenv("COLLECTOR_DB")
	if dbPath == "" {
		dbPath = "events.db"
	}
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("failed init sqlite: %v", err)
	}
	defer store.Close()

	col := collector.NewCollector(store)

	r := gin.Default()

	// Optional simple API token: set API_TOKEN env to enable.
	apiToken := os.Getenv("API_TOKEN")
	if apiToken != "" {
		// middleware to require X-API-Key header
		r.Use(func(c *gin.Context) {
			key := c.GetHeader("X-API-Key")
			if key == "" || key != apiToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			c.Next()
		})
	}

	// Ingest endpoint: singbox/xray 或 agent 将连接事件以 JSON POST 到这里
	r.POST("/ingest", func(c *gin.Context) {
		if err := col.HandleIngest(c.Request, c.Writer); err != nil {
			// HandleIngest 已经写入 response on success
			// 如果这里返回，表示有错误
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
	})

	// Simple API to query recent connections (PoC)
	r.GET("/api/v1/connections", func(c *gin.Context) {
		limit := 100
		events, err := store.ListRecent(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, events)
	})

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	addr := ":8080"
	if a := os.Getenv("LISTEN_ADDR"); a != "" {
		addr = a
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("collector listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
