package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/models"
	"go-novel/utils"
	"log"
	"strings"
	"sync"
	"time"
)

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
			if utils.IsEnglishSource(chapterJob.URL) {
				if strings.Contains(chapterJob.URL, "lightnovelworld.co") {
					jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, 9).Result()
					if err != nil {
						log.Printf("Error getting lightnovelworld.co chapter jobs: %v", err)
						time.Sleep(time.Second)
						continue
					}
				} else {
					jobs, err = w.Redis.LRange(ctx, chapterQueueKey, 0, 4).Result()
					if err != nil {
						log.Printf("Error getting chapter jobs: %v %e", chapterJob.URL, err)
						time.Sleep(time.Second)
						continue
					}
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
							if err := w.Redis.LRem(ctx, chapterQueueKey, 1, jobData).Err(); err != nil {
								log.Printf("Error removing job from chapter queue: %v", err)
							}
						}
					}(job)
				}

				wg.Wait()
			} else if strings.Contains(chapterJob.URL, "69shu.me") {
				jobs, err := w.Redis.LRange(ctx, chapterQueueKey, 0, 1).Result()
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

	if !utils.IsEnglishSource(chapter.URL) && (chapter.Content == nil || *chapter.Content == "") {
		log.Printf("Chapter %s has no content", chapter.URL)
		return w.enqueueForRetry(chapterJob)
	}

	chapter.NovelID = chapterJob.NovelID

	var existingChapter models.Chapter
	result := w.DB.Where("novel_id = ? AND number = ?", chapter.NovelID, chapter.Number).First(&existingChapter)

	if result.Error == nil {
		existingChapter.Title = chapter.Title
		existingChapter.Content = chapter.Content
		existingChapter.URL = chapter.URL

		if utils.IsEnglishSource(chapterJob.URL) {
			existingChapter.Slug = utils.Slugify(*existingChapter.TranslatedTitle)
			existingChapter.TranslatedContent = chapter.TranslatedContent
			existingChapter.TranslationStatus = "completed"
		}

		if err := w.DB.Save(&existingChapter).Error; err != nil {
			log.Printf("Error updating existing chapter %s: %v", chapter.Title, err)
			return w.enqueueForRetry(chapterJob)
		}

		if !utils.IsEnglishSource(chapterJob.URL) {
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

func (w *Worker) enqueueForRetry(job ChapterJob) error {
	job.Retries++
	log.Printf("Retrying chapter %s (attempt %d)", job.Title, job.Retries)

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling retry job: %w", err)
	}

	return w.enqueue(retryQueueKey, string(jobData))
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

func (w *Worker) isChapterProcessed(ctx context.Context, novelID uint, chapterNumber int) (bool, error) {
	key := fmt.Sprintf("%s:%d:%d", finishedChaptersKey, novelID, chapterNumber)
	return w.Redis.SIsMember(ctx, finishedChaptersKey, key).Result()
}

func (w *Worker) markChapterProcessed(ctx context.Context, novelID uint, chapterNumber int) error {
	key := fmt.Sprintf("%s:%d:%d", finishedChaptersKey, novelID, chapterNumber)
	return w.Redis.SAdd(ctx, finishedChaptersKey, key).Err()
}
