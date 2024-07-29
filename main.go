package main

import (
	"context"
	"go-novel/config"
	"go-novel/crawler"
	"go-novel/db"
	handlers "go-novel/handler"
	"go-novel/models"
	"go-novel/worker"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

func main() {
	cfg := config.LoadConfig()

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&models.Novel{}, &models.Chapter{})

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	e := echo.New()
	novelHandler := &handlers.NovelHandler{DB: db}

	e.GET("/novels/:id", novelHandler.GetNovel)
	e.GET("/novels/:id/chapters", novelHandler.GetNovelChapters)
	e.GET("/novels", novelHandler.GetNovels)
	e.GET("/latest-novels", novelHandler.GetLatestNovels)
	e.GET("/novels/:novel_id/chapters/:number", novelHandler.GetChapterByID)
	e.GET("/novels/chapters-stats/:id", novelHandler.GetNovelTranslationStatus)
	e.DELETE("/novels/:id", novelHandler.DeleteNovelByID)
	e.GET("/search", novelHandler.SearchNovels)

	crawler := crawler.NewCrawler()
	w := worker.NewWorker(crawler, db, rdb)
	go w.Start(context.Background())

	e.POST("/crawl", func(c echo.Context) error {
		url := c.FormValue("url")
		err := w.EnqueueNovel(url)
		if err != nil {
			return c.String(500, "Failed to enqueue novel")
		}
		return c.String(200, "Novel queued for crawling")
	})

	e.Logger.Fatal(e.Start(":" + cfg.ServerPort))
}
