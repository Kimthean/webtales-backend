package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/crawler"
	"go-novel/lib"
	"go-novel/models"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

const (
	novelQueueKey       = "novel_queue"
	chapterQueueKey     = "chapter_queue"
	retryQueueKey       = "retry_queue"
	translationQueueKey = "translation_queue"
	finishedChaptersKey = "finished_chapters"

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

type NovelJob struct {
	URL     string `json:"url"`
	Retries int    `json:"retries"`
}

func (nj *NovelJob) GetURL() string    { return nj.URL }
func (nj *NovelJob) GetRetries() int   { return nj.Retries }
func (nj *NovelJob) IncrementRetries() { nj.Retries++ }

type ChapterJob struct {
	URL     string `json:"url"`
	NovelID uint   `json:"novel_id"`
	Title   string `json:"title"`
	Number  int    `json:"number"`
	Retries int    `json:"retries"`
}

func (cj *ChapterJob) GetURL() string    { return cj.URL }
func (cj *ChapterJob) GetRetries() int   { return cj.Retries }
func (cj *ChapterJob) IncrementRetries() { cj.Retries++ }

type TranslationJob struct {
	ChapterID uint   `json:"chapter_id"`
	Field     string `json:"field"` // "title" or "content"
	Text      string `json:"text"`
	Retries   int    `json:"retries"`
}

func (tj *TranslationJob) GetRetries() int   { return tj.Retries }
func (tj *TranslationJob) IncrementRetries() { tj.Retries++ }

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
}

func (w *Worker) processQueue(ctx context.Context, queueKey string, processor func(context.Context, string) error) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopping %s queue processing", queueKey)
			return
		default:
			result, err := w.Redis.BLPop(ctx, 5*time.Second, queueKey).Result()
			if err == redis.Nil {
				continue
			} else if err != nil {
				log.Printf("Error popping from %s queue: %v", queueKey, err)
				log.Println("ReQueueing to retry ")

				// Requeue the job
				if err := w.enqueue(retryQueueKey, result[1]); err != nil {
					log.Printf("Error requeueing job: %v", err)
				}
				continue
			}

			if err := processor(ctx, result[1]); err != nil {
				log.Printf("Error processing %s: %v", queueKey, err)
			} else {

				if err := w.Redis.LRem(ctx, queueKey, 1, result[1]).Err(); err != nil {
					log.Printf("Error removing job from %s queue: %v", queueKey, err)
				}
			}
		}
	}
}

func (w *Worker) processNovel(ctx context.Context, jobData string) error {
	var novelJob NovelJob
	if err := json.Unmarshal([]byte(jobData), &novelJob); err != nil {
		return fmt.Errorf("unmarshalling novel job: %w", err)
	}

	if w.Crawler == nil {
		return w.enqueueNovelForRetry(novelJob)
	}

	log.Printf("Crawling novel: %s", novelJob.URL)
	novel, err := w.Crawler.CrawlNovel(novelJob.URL)
	if err != nil {
		log.Printf("Error crawling novel: %v", err)
		return w.enqueueNovelForRetry(novelJob)
	}

	log.Println("Pinging Redis...")
	pong, err := w.Redis.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("Redis ping failed: %v", err)
	} else {
		log.Printf("Redis ping successful: %s", pong)
	}

	if !strings.Contains(novelJob.URL, "wuxiabox.com") || strings.Contains(novelJob.URL, "lightnovelworld.co") {
		var translateTitle, translateAuthor, translateDescription string

		if novel.Title != nil {
			translated := w.translateAsync(*novel.Title)
			if translated != nil { // Check if the result is not nil
				translateTitle = *translated
			}
		}

		if novel.Author != nil {
			translated := w.translateAsync(*novel.Author)
			if translated != nil { // Check if the result is not nil
				translateAuthor = *translated
			}
		}

		if novel.Description != nil {
			translated := w.translateAsync(*novel.Description)
			if translated != nil { // Check if the result is not nil
				translateDescription = *translated
			}
		}

		novel.RawTitle = novel.Title
		novel.Title = &translateTitle
		novel.Author = &translateAuthor
		novel.Description = &translateDescription
	}

	if err := w.DB.Create(novel).Error; err != nil {
		log.Printf("Error saving novel: %v", err)
		return w.enqueueNovelForRetry(novelJob)
	}

	log.Printf("Enqueueing %d chapters...", len(novel.Chapters))
	for i, chapter := range novel.Chapters {
		if err := w.EnqueueChapter(chapter.URL, novel.ID, chapter.Title, chapter.Number); err != nil {
			log.Printf("Error enqueuing chapter %d: %v", i, err)
		} else {
			log.Printf("Chapter %d enqueued successfully", i)
		}
	}

	return nil
}

func (w *Worker) enqueueNovelForRetry(job NovelJob) error {
	job.IncrementRetries()
	log.Printf("Retrying novel %s (attempt %d)", job.URL, job.GetRetries())

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling retry job: %w", err)
	}

	return w.enqueue(retryQueueKey, string(jobData))
}

func (w *Worker) processChapters(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping chapter queue processing")
			return
		default:
			// Get the first job in the queue
			result, err := w.Redis.LRange(ctx, chapterQueueKey, 0, 0).Result()
			if err != nil {
				log.Printf("Error getting chapter job: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if len(result) == 0 {
				time.Sleep(time.Second)
				continue
			}

			var chapterJob ChapterJob
			if err := json.Unmarshal([]byte(result[0]), &chapterJob); err != nil {
				log.Printf("Error unmarshalling chapter job: %v", err)
				continue
			}

			if strings.Contains(chapterJob.URL, "wuxiabox.com") {
				jobs, err := w.Redis.LRange(ctx, chapterQueueKey, 0, 4).Result()
				if err != nil {
					log.Printf("Error getting wuxiabox.com chapter jobs: %v", err)
					time.Sleep(time.Second)
					continue
				}

				var wg sync.WaitGroup
				for _, job := range jobs {
					wg.Add(1)
					go func(jobData string) {
						defer wg.Done()
						if err := w.semaphore.Acquire(ctx, 1); err != nil {
							log.Printf("Failed to acquire semaphore: %v", err)
							return
						}
						defer w.semaphore.Release(1)

						if err := w.processChapter(jobData); err != nil {
							log.Printf("Error processing wuxiabox.com chapter: %v", err)
						} else {
							// Job processed successfully, remove it from the queue
							if err := w.Redis.LRem(ctx, chapterQueueKey, 1, jobData).Err(); err != nil {
								log.Printf("Error removing job from chapter queue: %v", err)
							}
						}
					}(job)
				}

				wg.Wait()

			} else {
				jobs, err := w.Redis.LRange(ctx, chapterQueueKey, 0, maxConcurrent-1).Result()
				if err != nil {
					log.Printf("Error getting chapter jobs: %v", err)
					time.Sleep(time.Second)
					continue
				}

				var wg sync.WaitGroup
				for _, job := range jobs {
					wg.Add(1)
					go func(jobData string) {
						defer wg.Done()
						if err := w.semaphore.Acquire(ctx, 1); err != nil {
							log.Printf("Failed to acquire semaphore: %v", err)
							return
						}
						defer w.semaphore.Release(1)

						if err := w.processChapter(jobData); err != nil {
							log.Printf("Error processing chapter: %v", err)
						} else {
							// Job processed successfully, remove it from the queue
							if err := w.Redis.LRem(ctx, chapterQueueKey, 1, jobData).Err(); err != nil {
								log.Printf("Error removing job from chapter queue: %v", err)
							}
						}
					}(job)
				}

				wg.Wait()
			}
		}
	}
}

func (w *Worker) processChapter(jobData string) error {
	var chapterJob ChapterJob
	if err := json.Unmarshal([]byte(jobData), &chapterJob); err != nil {
		return fmt.Errorf("unmarshalling chapter job: %w", err)
	}

	processed, err := w.isChapterProcessed(context.Background(), chapterJob.NovelID, chapterJob.Number)
	if err != nil {
		log.Printf("Error checking if chapter is processed: %v", err)
	} else if processed {
		log.Printf("Chapter %d of novel %d already processed, skipping", chapterJob.Number, chapterJob.NovelID)
		return nil
	}

	chapter, err := w.Crawler.CrawlChapter(chapterJob.URL, chapterJob.Title, chapterJob.Number)
	if err != nil {
		log.Printf("Error crawling chapter %s: %v", chapterJob.Title, err)
		return w.enqueueForRetry(chapterJob)
	}

	log.Printf("Crawled chapter: %s (NovelID: %d, Number: %d)", chapter.Title, chapterJob.NovelID, chapter.Number)

	if chapter.Content == nil || *chapter.Content == "" {
		log.Printf("Chapter %s has no content", chapter.Title)
		return w.enqueueForRetry(chapterJob)
	}

	chapter.NovelID = chapterJob.NovelID

	var existingChapter models.Chapter
	result := w.DB.Where("novel_id = ? AND number = ?", chapter.NovelID, chapter.Number).First(&existingChapter)

	isEnglishSource := strings.Contains(chapterJob.URL, "wuxiabox.com") || strings.Contains(chapterJob.URL, "lightnovelworld.co")

	if result.Error == nil {
		existingChapter.Title = chapter.Title
		existingChapter.Content = chapter.Content
		existingChapter.URL = chapter.URL

		if isEnglishSource {
			existingChapter.TranslatedTitle = &chapter.Title
			existingChapter.TranslatedContent = chapter.Content
			existingChapter.TranslationStatus = "completed"
		}

		if err := w.DB.Save(&existingChapter).Error; err != nil {
			log.Printf("Error updating existing chapter %s: %v", chapter.Title, err)
			return w.enqueueForRetry(chapterJob)
		}

		if !isEnglishSource {
			if existingChapter.TranslatedTitle == nil {
				w.enqueueTranslation(existingChapter.ID, "title", existingChapter.Title)
			}
			if existingChapter.TranslatedContent == nil || *existingChapter.TranslatedContent == "" {
				w.enqueueTranslation(existingChapter.ID, "content", *existingChapter.Content)
			}
		}
	} else {
		log.Printf("Database error while checking for existing chapter %s: %v", chapter.Title, result.Error)
		return w.enqueueForRetry(chapterJob)
	}

	if err := w.markChapterProcessed(context.Background(), chapterJob.NovelID, chapter.Number); err != nil {
		log.Printf("Error marking chapter as processed: %v", err)
	}

	return nil
}

func (w *Worker) enqueueTranslation(chapterID uint, field, text string) error {
	job := TranslationJob{
		ChapterID: chapterID,
		Field:     field,
		Text:      text,
		Retries:   0,
	}
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling translation job: %w", err)
	}
	return w.enqueue(translationQueueKey, string(jobData))
}

func (w *Worker) processTranslationQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping translation queue processing")
			return
		default:
			result, err := w.Redis.BLPop(ctx, 5*time.Second, translationQueueKey).Result()
			if err == redis.Nil {
				continue
			} else if err != nil {
				log.Printf("Error popping from translation queue: %v", err)
				continue
			}

			var job TranslationJob
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				log.Printf("Error unmarshalling translation job: %v", err)
				continue
			}

			time.Sleep(1 * time.Second)

			translated, err := lib.Translate(job.Text)
			if err != nil {
				log.Printf("Error translating text for chapter %d, field %s: %v", job.ChapterID, job.Field, err)
				job.IncrementRetries()
				if job.GetRetries() < maxRetries {
					w.enqueueTranslation(job.ChapterID, job.Field, job.Text)
				}
				continue
			}
			defer func() {
				if r := recover(); r != nil {

					job.IncrementRetries()
					if job.GetRetries() < maxRetries {
						w.enqueueTranslation(job.ChapterID, job.Field, job.Text)
					}
				}
			}()

			var chapter models.Chapter
			if err := w.DB.First(&chapter, job.ChapterID).Error; err != nil {
				log.Printf("Error fetching chapter %d: %v", job.ChapterID, err)
				continue
			}

			switch job.Field {
			case "title":
				chapter.TranslatedTitle = translated
				chapter.TranslationStatus = "title_translated"
			case "content":
				chapter.TranslatedContent = translated
				chapter.TranslationStatus = "content_translated"
			default:
				log.Printf("Unknown field for translation: %s", job.Field)
				continue
			}

			if chapter.TranslatedTitle != nil && chapter.TranslatedContent != nil {
				chapter.TranslationStatus = "completed"
			} else if (chapter.TranslatedContent == nil || *chapter.TranslatedContent == "") || (chapter.TranslatedTitle == nil || *chapter.TranslatedTitle == "") {
				job.IncrementRetries()
				if job.GetRetries() < maxRetries {
					w.enqueueTranslation(job.ChapterID, job.Field, job.Text)
				}
			} else {
				chapter.TranslationStatus = "completed"
			}

			if err := w.DB.Save(&chapter).Error; err != nil {
				log.Printf("Error saving translated %s for chapter %d: %v", job.Field, job.ChapterID, err)
				job.IncrementRetries()
				if job.GetRetries() < maxRetries {
					w.enqueueTranslation(job.ChapterID, job.Field, job.Text)
				}
			} else {
				log.Printf("Successfully translated and saved %s for chapter %d", job.Field, job.ChapterID)
				// Job processed successfully, remove it from the queue
				if err := w.Redis.LRem(ctx, translationQueueKey, 1, result[1]).Err(); err != nil {
					log.Printf("Error removing job from translation queue: %v", err)
				}
			}
		}
	}
}

func (w *Worker) enqueueForRetry(job ChapterJob) error {
	job.Retries++
	log.Printf("Retrying chapter %s (attempt %d)", job.Title, job.Retries)

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling retry job: %w", err)
	}

	return w.enqueue(retryQueueKey, string(jobData))
}

func (w *Worker) EnqueueNovel(url string) error {
	job, err := json.Marshal(NovelJob{URL: url, Retries: 0})
	if err != nil {
		return fmt.Errorf("marshalling novel job: %w", err)
	}
	return w.enqueue(novelQueueKey, string(job))
}

func (w *Worker) EnqueueChapter(url string, novelID uint, title string, number int) error {
	processed, err := w.isChapterProcessed(context.Background(), novelID, number)
	if err != nil {
		log.Printf("Error checking if chapter is processed: %v", err)
	} else if processed {
		log.Printf("Chapter %d of novel %d already processed, not enqueueing", number, novelID)
		return nil
	}

	job, err := json.Marshal(struct {
		URL     string `json:"url"`
		NovelID uint   `json:"novel_id"`
		Title   string `json:"title"`
		Number  int    `json:"number"`
	}{
		URL:     url,
		NovelID: novelID,
		Title:   title,
		Number:  number,
	})
	if err != nil {
		return fmt.Errorf("marshalling chapter job: %w", err)
	}
	return w.enqueue(chapterQueueKey, string(job))
}

func (w *Worker) processRetryQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping retry queue processing")
			return
		default:
			result, err := w.Redis.BLPop(ctx, 30*time.Second, retryQueueKey).Result()
			if err == redis.Nil {
				continue
			} else if err != nil {
				log.Printf("Error popping from retry queue: %v", err)
				continue
			}

			var job Job
			var rawJob map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &rawJob); err != nil {
				log.Printf("Error unmarshalling retry job: %v", err)
				continue
			}

			// Determine job type based on presence of novel_id field
			if _, ok := rawJob["novel_id"]; ok {
				var chapterJob ChapterJob
				if err := json.Unmarshal([]byte(result[1]), &chapterJob); err != nil {
					log.Printf("Error unmarshalling chapter job: %v", err)
					continue
				}
				job = &chapterJob
			} else {
				var novelJob NovelJob
				if err := json.Unmarshal([]byte(result[1]), &novelJob); err != nil {
					log.Printf("Error unmarshalling novel job: %v", err)
					continue
				}
				job = &novelJob
			}

			if chapterJob, ok := job.(*ChapterJob); ok {
				if err := w.processChapter(result[1]); err != nil {
					log.Printf("Error processing retry for chapter %s: %v", chapterJob.Title, err)
					w.enqueueForRetry(*chapterJob)
				}
			} else if novelJob, ok := job.(*NovelJob); ok {
				if err := w.processNovel(ctx, result[1]); err != nil {
					log.Printf("Error processing retry for novel %s: %v", novelJob.URL, err)
					w.enqueueNovelForRetry(*novelJob)
				}
			}
		}
	}
}

func (w *Worker) enqueue(queueKey string, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Redis.RPush(ctx, queueKey, value).Err(); err != nil {
		return fmt.Errorf("enqueueing to %s: %w", queueKey, err)
	}
	return nil
}

func (w *Worker) translateAsync(content string) *string {
	resultChan := make(chan string, 1)

	go func() {
		translated, err := lib.Translate(content)
		if err != nil {
			resultChan <- ""
		} else {
			resultChan <- *translated
		}
		close(resultChan)
	}()

	result := <-resultChan
	return &result
}

func (w *Worker) isChapterProcessed(ctx context.Context, novelID uint, chapterNumber int) (bool, error) {
	key := fmt.Sprintf("%s:%d:%d", finishedChaptersKey, novelID, chapterNumber)
	return w.Redis.SIsMember(ctx, finishedChaptersKey, key).Result()
}

func (w *Worker) markChapterProcessed(ctx context.Context, novelID uint, chapterNumber int) error {
	key := fmt.Sprintf("%s:%d:%d", finishedChaptersKey, novelID, chapterNumber)
	return w.Redis.SAdd(ctx, finishedChaptersKey, key).Err()
}
