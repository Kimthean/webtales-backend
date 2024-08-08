package main

import (
	"context"
	"go-novel/config"
	"go-novel/crawler"
	"go-novel/db"
	handlers "go-novel/handler"
	"go-novel/worker"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		panic("failed to connect database")
	}
	// db.AutoMigrate(&models.Novel{}, &models.Chapter{})
	redisURL := cfg.RedisURL
	redisURL = strings.TrimPrefix(redisURL, "redis://")
	parts := strings.Split(redisURL, "@")
	if len(parts) != 2 {
		log.Fatalf("Invalid Redis URL format: %s", cfg.RedisURL)
	}
	password := strings.TrimPrefix(parts[0], ":")
	address := parts[1]

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	// err = utils.InitS3()
	// if err != nil {
	// 	log.Fatalf("Failed to initialize S3: %v", err)
	// }

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	novelHandler := &handlers.NovelHandler{DB: db}

	r.GET("/novels/:id", novelHandler.GetNovel)
	r.GET("/novels/:id/chapters", novelHandler.GetNovelChapters)
	r.GET("/novels/all", novelHandler.GetNovels)
	r.GET("/novels", novelHandler.GetPaginatedNovels)
	r.GET("/latest-novels", novelHandler.GetLatestNovels)
	r.GET("/novels/:id/chapter/:number", novelHandler.GetChapterByID)
	r.GET("/novels/chapters-stats/:id", novelHandler.GetNovelTranslationStatus)
	r.DELETE("/novels/:id", novelHandler.DeleteNovelByID)
	r.GET("/search", novelHandler.SearchNovels)
	r.GET("/health", func(c *gin.Context) {
		var err error
		var version string
		err = db.Raw("SELECT VERSION()").Scan(&version).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database is unreachable"})
			return
		}

		// Redis check
		if _, err = rdb.Ping(context.Background()).Result(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis is unreachable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "All services are healthy!",
		})
	})

	crawler := crawler.NewCrawler()
	w := worker.NewWorker(crawler, db, rdb)
	go w.Start(context.Background())

	r.POST("/crawl", func(c *gin.Context) {
		url := c.Query("url")
		log.Printf("Crawling %s", url)
		err := w.EnqueueNovel(url)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to enqueue novel")
			return
		}
		c.String(http.StatusOK, "Novel queued for crawling")
	})

	r.Run(":" + cfg.ServerPort)
}
