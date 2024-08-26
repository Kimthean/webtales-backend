package worker

import (
	"go-novel/models"
	"log"
)

func (w *Worker) checkMissingTranslations(completedJobs map[uint]bool) {
	var chapters []models.Chapter
	if err := w.DB.Where("id IN ?", keysToSlice(completedJobs)).Find(&chapters).Error; err != nil {
		log.Printf("Error fetching chapters: %v", err)
		return
	}

	for _, chapter := range chapters {
		if chapter.TranslatedTitle == nil || chapter.TranslatedContent == nil || chapter.TranslationStatus != "completed" {
			log.Printf("Chapter %d has missing translations. Enqueueing for retranslation.", chapter.ID)
			w.enqueueTranslation(chapter.ID, "title", chapter.Title)
			w.enqueueTranslation(chapter.ID, "content", *chapter.Content)
		}
	}
}

func keysToSlice(m map[uint]bool) []uint {
	keys := make([]uint, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
