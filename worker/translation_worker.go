package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/lib"
	"go-novel/models"
	"go-novel/utils"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type TranslationJob struct {
	ChapterID uint   `json:"chapter_id"`
	Field     string `json:"field"`
	Text      string `json:"text"`
	Retries   int    `json:"retries"`
}

func (tj *TranslationJob) GetRetries() int   { return tj.Retries }
func (tj *TranslationJob) IncrementRetries() { tj.Retries++ }

func (w *Worker) processTranslationQueue(ctx context.Context) {
	completedJobs := make(map[uint]bool)
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping translation queue processing")
			return
		default:
			result, err := w.Redis.BLPop(ctx, 1*time.Second, translationQueueKey).Result()
			if err == redis.Nil {
				if len(completedJobs) > 0 {
					w.checkMissingTranslations(completedJobs)
					completedJobs = make(map[uint]bool)
				}
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

			// if job.Text == "" {
			// 	log.Printf("Skipping translation for chapter %d, field %s due to empty text", job.ChapterID, job.Field)
			// 	completedJobs[job.ChapterID] = true
			// 	continue
			// }

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
				slug := utils.Slugify(*translated)
				chapter.Slug = slug
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
			completedJobs[job.ChapterID] = true
		}
	}
}

func (w *Worker) enqueueTranslation(chapterID uint, field, text string) error {
	job := TranslationJob{
		ChapterID: chapterID,
		Field:     field,
		Text:      text,
		Retries:   3,
	}
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling translation job: %w", err)
	}
	return w.enqueue(translationQueueKey, string(jobData))
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

func (w *Worker) RetranslateChapters(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
	}()

	var chapters []models.Chapter
	if err := w.DB.Where("(translated_content IS NULL OR translated_title IS NULL OR translation_status IS NULL) OR translation_status <> 'completed'").Find(&chapters).Error; err != nil {
		log.Printf("Failed to fetch chapters with incomplete translations: %v", err)
		return nil
	}

	log.Printf("Processing %d chapters for retranslation...", len(chapters))

	for _, chapter := range chapters {
		log.Printf("Processing chapter %s for retranslation", chapter.Title)

		if chapter.TranslatedTitle == nil || *chapter.TranslatedTitle == "" {
			if err := w.enqueueTranslation(chapter.ID, "title", chapter.Title); err != nil {
				log.Printf("Error enqueueing title translation for chapter %d: %v", chapter.ID, err)
				continue
			}
		}

		if chapter.TranslatedContent == nil || *chapter.TranslatedContent == "" {
			log.Printf("Processing content translation for chapter %d", chapter.ID)

			if err := w.enqueueTranslation(chapter.ID, "content", *chapter.Content); err != nil {
				log.Printf("Error enqueueing content translation for chapter %d: %v", chapter.ID, err)
				continue
			}

		}
	}

	return nil
}
