package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/utils"
	"log"
)

type NovelJob struct {
	URL     string `json:"url"`
	Retries int    `json:"retries"`
}

func (nj *NovelJob) GetURL() string    { return nj.URL }
func (nj *NovelJob) GetRetries() int   { return nj.Retries }
func (nj *NovelJob) IncrementRetries() { nj.Retries++ }

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

	if novel.Thumbnail != nil {
		s3URL, err := utils.DownloadAndUploadImage(*novel.Thumbnail, "cover")
		if err != nil {
			log.Printf("Error downloading or uploading thumbnail: %v", err)
		} else {
			novel.Thumbnail = &s3URL
		}
	}

	log.Println("Pinging Redis...")
	pong, err := w.Redis.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("Redis ping failed: %v", err)
	} else {
		log.Printf("Redis ping successful: %s", pong)
	}

	if !utils.IsEnglishSource(novelJob.URL) {
		var translateTitle, translateDescription string

		if novel.Title != nil {
			translated := w.translateAsync(*novel.Title)
			if translated != nil {
				translateTitle = *translated
			}
		}

		if novel.Description != nil {
			translated := w.translateAsync(*novel.Description)
			if translated != nil {
				translateDescription = *translated
			}
		}

		novel.RawTitle = novel.Title
		novel.Title = &translateTitle
		novel.Description = &translateDescription
		slug := utils.Slugify(*novel.Title)
		novel.Slug = &slug
	}
	slug := utils.Slugify(*novel.Title)
	novel.Slug = &slug

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

func (w *Worker) EnqueueNovel(url string) error {
	job, err := json.Marshal(NovelJob{URL: url, Retries: 0})
	if err != nil {
		return fmt.Errorf("marshalling novel job: %w", err)
	}
	return w.enqueue(novelQueueKey, string(job))
}
