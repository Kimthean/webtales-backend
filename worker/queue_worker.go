package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

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

func (w *Worker) enqueue(queueKey string, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Redis.RPush(ctx, queueKey, value).Err(); err != nil {
		return fmt.Errorf("enqueueing to %s: %w", queueKey, err)
	}

	return nil
}
