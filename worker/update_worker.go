package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/models"
	"log"
	"strconv"

	"gorm.io/gorm"
)

type UpdateJob struct {
	NovelID uint `json:"novel_id"`
	Retries int  `json:"retries"`
}

func (uj *UpdateJob) GetRetries() int   { return uj.Retries }
func (uj *UpdateJob) IncrementRetries() { uj.Retries++ }

func (w *Worker) processUpdate(ctx context.Context, jobData string) error {
	var updateJob UpdateJob
	if err := json.Unmarshal([]byte(jobData), &updateJob); err != nil {
		return fmt.Errorf("unmarshalling update job: %w", err)
	}

	var novelURL string
	var existingNovel models.Novel
	result := w.DB.First(&existingNovel, updateJob.NovelID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			log.Printf("Novel with ID %d not found.", updateJob.NovelID)
			return fmt.Errorf("novel not found")
		} else if result.Error != nil {
			log.Printf("Error fetching novel: %v", result.Error)
			return result.Error
		}
	}
	novelURL = *existingNovel.URL

	novel, err := w.Crawler.CrawlNovel(novelURL)
	if err != nil {
		log.Printf("Error crawling novel: %v", err)
		return err
	}

	log.Printf("Crawled novel: %s", strconv.Itoa(int(existingNovel.ID)))
	var existingChapters []models.Chapter
	result = w.DB.Where("novel_id = ?", existingNovel.ID).Find(&existingChapters)
	if result.Error != nil {
		log.Printf("Error fetching existing chapters for novel ID %d: %v", existingNovel.ID, result.Error)
		return result.Error
	}
	log.Printf("Fetched %d existing chapters", len(existingChapters))

	existingChapterNumbers := make(map[int]bool)
	for _, chapter := range existingChapters {
		existingChapterNumbers[chapter.Number] = true
	}

	for _, chapter := range novel.Chapters {
		if _, exists := existingChapterNumbers[chapter.Number]; !exists {
			newChapter := models.Chapter{
				NovelID:           existingNovel.ID,
				Number:            chapter.Number,
				TranslatedTitle:   chapter.TranslatedTitle,
				URL:               chapter.URL,
				TranslationStatus: "pending",
			}
			if err := w.DB.Create(&newChapter).Error; err != nil {
				log.Printf("Error saving new chapter %d: %v", chapter.Number, err)
				continue
			}
			log.Printf("New chapter %d saved successfully", chapter.Number)

			if err := w.EnqueueChapter(chapter.URL, existingNovel.ID, chapter.Title, chapter.Number); err != nil {
				log.Printf("Error enqueuing chapter %d: %v", chapter.Number, err)
			} else {
				log.Printf("New chapter %d enqueued successfully", chapter.Number)
			}
		}
	}

	return nil
}

func (w *Worker) EnqueueUpdate(novelID uint) error {
	job := UpdateJob{
		NovelID: novelID,
		Retries: 0,
	}
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling update job: %w", err)
	}

	return w.enqueue(updateQueueKey, string(jobData))
}
