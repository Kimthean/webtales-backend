package worker

import (
	"context"
	"go-novel/crawler"
	"log"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

const (
	novelQueueKey          = "novel_queue"
	chapterQueueKey        = "chapter_queue"
	retryQueueKey          = "retry_queue"
	translationQueueKey    = "translation_queue"
	finishedChaptersKey    = "finished_chapters"
	updateQueueKey         = "update_queue"
	epubConversionQueueKey = "epub_conversion_queue"

	maxRetries    = 5
	maxConcurrent = 20
)

type Worker struct {
	DB        *gorm.DB
	Redis     *redis.Client
	Crawler   *crawler.Crawler
	semaphore *semaphore.Weighted
}

type Job interface {
	GetURL() string
	GetRetries() int
	IncrementRetries()
}

func NewWorker(crawler *crawler.Crawler, db *gorm.DB, redis *redis.Client) *Worker {
	if crawler == nil {
		log.Fatal("Crawler cannot be nil")
	}
	semaphore := semaphore.NewWeighted(maxConcurrent)
	if semaphore == nil {
		log.Panic("Failed to initialize semaphore")
	}
	return &Worker{
		DB:        db,
		Redis:     redis,
		Crawler:   crawler,
		semaphore: semaphore,
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w.semaphore == nil {
		log.Fatal("Semaphore is nil. Worker not properly initialized.")
	}

	if w.Redis == nil {
		log.Fatal("Redis client is nil. Worker not properly initialized.")
	}

	go w.processQueue(ctx, novelQueueKey, w.processNovel)
	go w.processChapters(ctx)
	go w.processRetryQueue(ctx)
	go w.processTranslationQueue(ctx)
	go w.processQueue(ctx, updateQueueKey, w.processUpdate)
	go w.processEPUBConversionQueue(ctx)

}
