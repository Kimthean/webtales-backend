package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

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

			var rawJob map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &rawJob); err != nil {
				log.Printf("Error unmarshalling retry job: %v", err)
				continue
			}

			// Determine job type based on presence of novel_id field
			if _, ok := rawJob["novel_id"]; ok {
				if _, ok := rawJob["title"]; ok {
					var chapterJob ChapterJob
					if err := json.Unmarshal([]byte(result[1]), &chapterJob); err != nil {
						log.Printf("Error unmarshalling chapter job: %v", err)
						continue
					}
					if err := w.processChapter(result[1]); err != nil {
						log.Printf("Error processing retry for chapter %s: %v", chapterJob.Title, err)
						w.enqueueForRetry(chapterJob)
					}
				} else {
					var updateJob UpdateJob
					if err := json.Unmarshal([]byte(result[1]), &updateJob); err != nil {
						log.Printf("Error unmarshalling update job: %v", err)
						continue
					}
					if err := w.processUpdate(ctx, result[1]); err != nil {
						log.Printf("Error processing retry for update %d: %v", updateJob.NovelID, err)
						w.enqueueUpdateForRetry(updateJob)
					}
				}
			} else {
				var novelJob NovelJob
				if err := json.Unmarshal([]byte(result[1]), &novelJob); err != nil {
					log.Printf("Error unmarshalling novel job: %v", err)
					continue
				}
				if err := w.processNovel(ctx, result[1]); err != nil {
					log.Printf("Error processing retry for novel %s: %v", novelJob.URL, err)
					w.enqueueNovelForRetry(novelJob)
				}
			}
		}
	}
}

func (w *Worker) enqueueUpdateForRetry(job UpdateJob) error {
	job.IncrementRetries()
	log.Printf("Retrying update for novel %d (attempt %d)", job.NovelID, job.GetRetries())

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshalling retry job: %w", err)
	}

	return w.enqueue(retryQueueKey, string(jobData))
}
