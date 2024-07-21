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
	maxRetries          = 5
	maxConcurrent       = 10
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

	log.Printf("Total chapters to queue: %d", len(novel.Chapters))

	log.Println("Pinging Redis...")
	pong, err := w.Redis.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("Redis ping failed: %v", err)
	} else {
		log.Printf("Redis ping successful: %s", pong)
	}

	log.Println("Starting title translation...")
	translateTitle := *w.translateAsync(*novel.Title)
	log.Printf("Translated Title: %s", translateTitle)

	log.Println("Starting author translation...")
	translateAuthor := *w.translateAsync(*novel.Author)
	log.Printf("Translated Author: %s", translateAuthor)

	log.Println("Starting description translation...")
	translateDescription := *w.translateAsync(*novel.Description)
	log.Printf("Translated Description: %s", translateDescription)

	if novel.Title != nil {
		translateTitle = *w.translateAsync(*novel.Title)
		log.Printf("Translated Title: %s", translateTitle)
	} else {
		log.Println("Novel title is nil")
	}

	if novel.Author != nil {
		translateAuthor = *w.translateAsync(*novel.Author)
		log.Printf("Translated Author: %s", translateAuthor)
	} else {
		log.Println("Novel author is nil")
	}

	if novel.Description != nil {
		translateDescription = *w.translateAsync(*novel.Description)
		log.Printf("Translated Description: %s", translateDescription)
	} else {
		log.Println("Novel description is nil")
	}

	novel.RawTitle = novel.Title
	novel.Title = &translateTitle
	novel.Author = &translateAuthor
	novel.Description = &translateDescription

	log.Printf("Crawled novel: %s", *novel.Title)
	log.Printf("Crawled novel author: %s", *novel.Author)
	log.Printf("Crawled novel description: %s", *novel.Description)

	log.Println("Attempting to save novel to database...")
	if err := w.DB.Create(novel).Error; err != nil {
		log.Printf("Error saving novel: %v", err)
		return w.enqueueNovelForRetry(novelJob)
	}
	log.Println("Novel saved successfully to database")

	log.Printf("Enqueueing %d chapters...", len(novel.Chapters))
	for i, chapter := range novel.Chapters {
		if err := w.EnqueueChapter(chapter.URL, novel.ID, chapter.Title, chapter.Number); err != nil {
			log.Printf("Error enqueuing chapter %d: %v", i, err)
		} else {
			log.Printf("Chapter %d enqueued successfully", i)
		}
	}
	log.Println("All chapters enqueued")

	log.Println("Novel processing completed successfully")

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

			var jobs []string
			if strings.Contains(chapterJob.URL, "wuxiabox.com") {
				jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, 5).Result()
			} else {
				jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, maxConcurrent-1).Result()
			}

			if err != nil {
				log.Printf("Error getting chapter jobs: %v", err)
				time.Sleep(time.Second)
				continue
			}

			// Filter out recently processed chapters
			var filteredJobs []ChapterJob
			pipe := w.Redis.Pipeline()
			for _, job := range jobs {
				var cj ChapterJob
				if err := json.Unmarshal([]byte(job), &cj); err != nil {
					log.Printf("Error unmarshalling chapter job: %v", err)
					continue
				}
				cacheKey := fmt.Sprintf("processed_chapter:%d:%d", cj.NovelID, cj.Number)
				pipe.Exists(ctx, cacheKey)
			}
			cmders, err := pipe.Exec(ctx)
			if err != nil {
				log.Printf("Error checking cache for processed chapters: %v", err)
				continue
			}

			for i, cmder := range cmders {
				exists, err := cmder.(*redis.IntCmd).Result()
				if err != nil {
					log.Printf("Error checking cache result: %v", err)
					continue
				}
				if exists == 0 {
					var cj ChapterJob
					if err := json.Unmarshal([]byte(jobs[i]), &cj); err != nil {
						log.Printf("Error unmarshalling chapter job: %v", err)
						continue
					}
					filteredJobs = append(filteredJobs, cj)
				}
			}

			processedChapters := w.processBulkChapters(ctx, filteredJobs)

			// Update database in a single transaction
			err = w.DB.Transaction(func(tx *gorm.DB) error {
				for _, chapter := range processedChapters {
					result := tx.Model(&models.Chapter{}).
						Where("novel_id = ? AND number = ?", chapter.NovelID, chapter.Number).
						Updates(map[string]interface{}{
							"title":              chapter.Title,
							"content":            chapter.Content,
							"url":                chapter.URL,
							"translated_title":   chapter.TranslatedTitle,
							"translated_content": chapter.TranslatedContent,
							"translation_status": chapter.TranslationStatus,
						})

					if result.Error != nil {
						return result.Error
					}

					if result.RowsAffected == 0 {
						if err := tx.Create(&chapter).Error; err != nil {
							return err
						}
					}
				}
				return nil
			})

			if err != nil {
				log.Printf("Error updating chapters in bulk: %v", err)
			} else {
				log.Printf("Successfully updated %d chapters in bulk", len(processedChapters))
			}

			// Set cache for processed chapters
			pipe = w.Redis.Pipeline()
			for _, chapter := range processedChapters {
				cacheKey := fmt.Sprintf("processed_chapter:%d:%d", chapter.NovelID, chapter.Number)
				pipe.Set(ctx, cacheKey, "1", 1*time.Hour)
			}
			_, err = pipe.Exec(ctx)
			if err != nil {
				log.Printf("Error setting cache for processed chapters: %v", err)
			}

			// Remove all processed jobs from the queue, including skipped ones
			if _, err := w.Redis.LTrim(ctx, chapterQueueKey, int64(len(jobs)), -1).Result(); err != nil {
				log.Printf("Error trimming processed jobs from queue: %v", err)
			}
		}
	}
}

func (w *Worker) processBulkChapters(ctx context.Context, jobs []ChapterJob) []models.Chapter {
	var processedChapters []models.Chapter
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, job := range jobs {
		wg.Add(1)
		go func(cj ChapterJob) {
			defer wg.Done()
			if err := w.semaphore.Acquire(ctx, 1); err != nil {
				log.Printf("Failed to acquire semaphore: %v", err)
				return
			}
			defer w.semaphore.Release(1)

			chapter, err := w.Crawler.CrawlChapter(cj.URL, cj.Title, cj.Number)
			if err != nil {
				log.Printf("Error crawling chapter %s: %v", cj.Title, err)
				return
			}

			chapter.NovelID = cj.NovelID

			if strings.Contains(cj.URL, "wuxiabox.com") {
				chapter.TranslatedTitle = &chapter.Title
				chapter.TranslatedContent = chapter.Content
				chapter.TranslationStatus = "completed"
			} else {
				// Enqueue for translation if needed
				w.enqueueTranslation(chapter.ID, "title", chapter.Title)
				w.enqueueTranslation(chapter.ID, "content", *chapter.Content)
			}

			mu.Lock()
			processedChapters = append(processedChapters, *chapter)
			mu.Unlock()
		}(job)
	}

	wg.Wait()
	return processedChapters
}

// func (w *Worker) processChapters(ctx context.Context) {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Println("Stopping chapter queue processing")
// 			return
// 		default:
// 			// Get the first job in the queue
// 			result, err := w.Redis.LRange(ctx, chapterQueueKey, 0, 0).Result()
// 			if err != nil {
// 				log.Printf("Error getting chapter job: %v", err)
// 				time.Sleep(time.Second)
// 				continue
// 			}

// 			if len(result) == 0 {
// 				time.Sleep(time.Second)
// 				continue
// 			}

// 			var chapterJob ChapterJob
// 			if err := json.Unmarshal([]byte(result[0]), &chapterJob); err != nil {
// 				log.Printf("Error unmarshalling chapter job: %v", err)
// 				continue
// 			}

// 			var jobs []string
// 			if strings.Contains(chapterJob.URL, "wuxiabox.com") {
// 				jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, 4).Result()
// 			} else {
// 				jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, maxConcurrent-1).Result()
// 			}

// 			if err != nil {
// 				log.Printf("Error getting chapter jobs: %v", err)
// 				time.Sleep(time.Second)
// 				continue
// 			}

// 			// Filter out recently processed chapters
// 			var filteredJobs []string
// 			pipe := w.Redis.Pipeline()
// 			for _, job := range jobs {
// 				var cj ChapterJob
// 				if err := json.Unmarshal([]byte(job), &cj); err != nil {
// 					log.Printf("Error unmarshalling chapter job: %v", err)
// 					continue
// 				}
// 				cacheKey := fmt.Sprintf("processed_chapter:%d:%d", cj.NovelID, cj.Number)
// 				pipe.Exists(ctx, cacheKey)
// 			}
// 			cmders, err := pipe.Exec(ctx)
// 			if err != nil {
// 				log.Printf("Error checking cache for processed chapters: %v", err)
// 				continue
// 			}

// 			for i, cmder := range cmders {
// 				exists, err := cmder.(*redis.IntCmd).Result()
// 				if err != nil {
// 					log.Printf("Error checking cache result: %v", err)
// 					continue
// 				}
// 				if exists == 0 {
// 					filteredJobs = append(filteredJobs, jobs[i])
// 				}
// 			}

// 			var wg sync.WaitGroup
// 			for _, job := range filteredJobs {
// 				wg.Add(1)
// 				go func(jobData string) {
// 					defer wg.Done()
// 					if err := w.semaphore.Acquire(ctx, 1); err != nil {
// 						log.Printf("Failed to acquire semaphore: %v", err)
// 						return
// 					}
// 					defer w.semaphore.Release(1)

// 					if err := w.processChapter(jobData); err != nil {
// 						log.Printf("Error processing chapter: %v", err)
// 					}
// 				}(job)
// 			}

// 			wg.Wait()

// 			// Remove all processed jobs from the queue, including skipped ones
// 			if _, err := w.Redis.LTrim(ctx, chapterQueueKey, int64(len(jobs)), -1).Result(); err != nil {
// 				log.Printf("Error trimming processed jobs from queue: %v", err)
// 			}
// 		}
// 	}
// }

func (w *Worker) processChapter(jobData string) error {
	var chapterJob ChapterJob
	if err := json.Unmarshal([]byte(jobData), &chapterJob); err != nil {
		return fmt.Errorf("unmarshalling chapter job: %w", err)
	}

	lockKey := fmt.Sprintf("chapter_lock:%d:%d", chapterJob.NovelID, chapterJob.Number)
	cacheKey := fmt.Sprintf("processed_chapter:%d:%d", chapterJob.NovelID, chapterJob.Number)
	ctx := context.Background()

	// Try to acquire the lock
	locked, err := w.Redis.SetNX(ctx, lockKey, "1", 5*time.Minute).Result()
	if err != nil {
		log.Printf("Error acquiring lock: %v", err)
		return w.enqueueForRetry(chapterJob)
	}
	if !locked {
		log.Printf("Chapter %d for novel %d is being processed by another worker, skipping", chapterJob.Number, chapterJob.NovelID)
		return nil
	}
	defer w.Redis.Del(ctx, lockKey)

	// Check if chapter was recently processed
	exists, err := w.Redis.Exists(ctx, cacheKey).Result()
	if err != nil {
		log.Printf("Error checking cache: %v", err)
	} else if exists == 1 {
		log.Printf("Chapter %d for novel %d was recently processed, skipping", chapterJob.Number, chapterJob.NovelID)
		return nil
	}

	log.Printf("Crawling chapter: %s (NovelID: %d, Number: %d)", chapterJob.Title, chapterJob.NovelID, chapterJob.Number)

	chapter, err := w.Crawler.CrawlChapter(chapterJob.URL, chapterJob.Title, chapterJob.Number)
	if err != nil {
		log.Printf("Error crawling chapter %s: %v", chapterJob.Title, err)
		return w.enqueueForRetry(chapterJob)
	}

	if chapter.Content == nil || *chapter.Content == "" {
		return w.enqueueForRetry(chapterJob)
	}

	chapter.NovelID = chapterJob.NovelID

	// Use a single query to check if the chapter exists and update or insert accordingly
	result := w.DB.Model(&models.Chapter{}).
		Where("novel_id = ? AND number = ?", chapter.NovelID, chapter.Number).
		Updates(models.Chapter{
			Title:   chapter.Title,
			Content: chapter.Content,
			URL:     chapter.URL,
		})

	if result.Error != nil {
		log.Printf("Error upserting chapter %s: %v", chapterJob.Title, result.Error)
		return w.enqueueForRetry(chapterJob)
	}

	if result.RowsAffected == 0 {
		// Chapter doesn't exist, create it
		newChapter := models.Chapter{
			NovelID: chapter.NovelID,
			Number:  chapter.Number,
			Title:   chapter.Title,
			Content: chapter.Content,
			URL:     chapter.URL,
		}

		if err := w.DB.Create(&newChapter).Error; err != nil {
			log.Printf("Error creating new chapter %s: %v", chapterJob.Title, err)
			return w.enqueueForRetry(chapterJob)
		}
	}

	log.Printf("Successfully processed chapter: %s (NovelID: %d, Number: %d)", chapter.Title, chapter.NovelID, chapter.Number)

	// After successful processing, add to cache
	if err := w.Redis.Set(ctx, cacheKey, "1", 1*time.Hour).Err(); err != nil {
		log.Printf("Error setting cache: %v", err)
	}

	// Enqueue for translation if needed
	if !strings.Contains(chapterJob.URL, "wuxiabox.com") {
		w.enqueueTranslation(chapter.ID, "title", chapter.Title)
		w.enqueueTranslation(chapter.ID, "content", *chapter.Content)
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
