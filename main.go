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

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func main() {
	cfg := config.LoadConfig()

	gin.SetMode(gin.ReleaseMode)

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		panic("failed to connect database")
	}
	// db.AutoMigrate(&models.Novel{}, &models.Chapter{})

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	// err = utils.InitS3()
	// if err != nil {
	// 	log.Fatalf("Failed to initialize S3: %v", err)
	// }

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
