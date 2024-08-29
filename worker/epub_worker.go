package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"go-novel/models"
	"go-novel/utils"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-shiori/go-epub"
)

func (w *Worker) processEPUBConversionQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping EPUB conversion queue processing")
			return
		default:
			result, err := w.Redis.BLPop(ctx, 5*time.Second, epubConversionQueueKey).Result()
			if err == redis.Nil {
				continue
			} else if err != nil {
				log.Printf("Error popping from EPUB conversion queue: %v", err)
				continue
			}

			var job map[string]string
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				log.Printf("Error unmarshalling conversion job: %v", err)
				continue
			}

			novelID := job["novel_id"]
			err = w.ConvertNovelToEPUB(ctx, novelID)
			if err != nil {
				log.Printf("Error converting novel %s to EPUB: %v", novelID, err)
				// Optionally, handle retries or log the error
			}
		}
	}
}

func (w *Worker) ConvertNovelToEPUB(ctx context.Context, novelID string) error {
	var novel models.Novel
	if err := w.DB.First(&novel, novelID).Error; err != nil {
		return fmt.Errorf("failed to fetch novel: %v", err)
	}

	var chapters []models.Chapter
	if err := w.DB.Where("novel_id = ?", novelID).Order("number ASC").Find(&chapters).Error; err != nil {
		return fmt.Errorf("failed to fetch chapters: %v", err)
	}

	if novel.Title == nil {
		return fmt.Errorf("novel title is nil")
	}
	e, err := epub.NewEpub(*novel.Title)
	if err != nil {
		log.Printf("EPub Error")
	}
	if novel.Author != nil {
		e.SetAuthor(*novel.Author)
	} else {
		e.SetAuthor("Unknown")
	}

	if novel.Description != nil {
		e.SetDescription(*novel.Description)
	} else {
		e.SetDescription("No description available")
	}

	if novel.Thumbnail != nil {
		resp, err := http.Get(*novel.Thumbnail)
		if err != nil {
			return fmt.Errorf("failed to download thumbnail: %v", err)
		}
		defer resp.Body.Close()

		thumbnailFile, err := os.CreateTemp("", "thumbnail-*.jpg")
		if err != nil {
			return fmt.Errorf("failed to create temporary file for thumbnail: %v", err)
		}
		defer os.Remove(thumbnailFile.Name())

		_, err = io.Copy(thumbnailFile, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to save thumbnail: %v", err)
		}

		_, err = e.AddImage(thumbnailFile.Name(), "")
		if err != nil {
			return fmt.Errorf("failed to add cover image: %v", err)
		}
		e.SetCover(thumbnailFile.Name(), "")
	}

	watermarkText := "This EPUB is downloaded from WebtalesMTL. Please visit https://webtalesmtl.xyz for more novels."

	for _, chapter := range chapters {
		var content string
		if chapter.TranslatedContent != nil {
			content = *chapter.TranslatedContent
		} else if chapter.Content != nil {
			content = *chapter.Content
		}
		if content == "" {
			log.Printf("Chapter %d has no content, skipping", chapter.ID)
			continue
		}

		contentWithWatermark := content + watermarkText
		contentWithParagraphs := "<p>" + strings.ReplaceAll(html.EscapeString(contentWithWatermark), "\n\n", "</p><p>") + "</p>"

		var chapterTitle string
		if chapter.TranslatedTitle != nil {
			chapterTitle = html.EscapeString(*chapter.TranslatedTitle)
		} else {
			chapterTitle = "Chapter " + strconv.Itoa(chapter.Number)
		}

		contentWithTitle := "<h2>" + chapterTitle + "</h2>" + contentWithParagraphs
		_, err := e.AddSection(contentWithTitle, chapterTitle, "", "")
		if err != nil {
			return fmt.Errorf("failed to add chapter %d to EPUB: %v", chapter.ID, err)
		}
	}

	currentDate := time.Now().Format("2006-01-02")

	if novel.Title == nil {
		return fmt.Errorf("novel title is nil")
	}
	filename := fmt.Sprintf("%s-%s", utils.Slugify(*novel.Title), currentDate)
	destFilePath := fmt.Sprintf("%s.epub", filename)
	if err := e.Write(destFilePath); err != nil {
		return fmt.Errorf("failed to write EPUB file: %v", err)
	}

	var s3Url string
	s3Url, err = utils.UploadFileToS3(destFilePath, "epub", "epub")
	if err != nil {
		log.Println("Failed to upload EPUB to S3")
	}

	w.DB.Model(&models.Novel{}).Where("id = ?", novel.ID).Updates(map[string]interface{}{"epub_url": s3Url})

	os.Remove(destFilePath)

	return nil
}

func (w *Worker) EnqueueNovelForConversion(novelID string) error {
	// Marshal the novel ID into a job format suitable for your queue
	jobData, err := json.Marshal(map[string]string{"novel_id": novelID})
	if err != nil {
		return fmt.Errorf("marshalling conversion job: %w", err)
	}

	// Push the job to the Redis queue
	return w.Redis.RPush(context.Background(), epubConversionQueueKey, string(jobData)).Err()
}
