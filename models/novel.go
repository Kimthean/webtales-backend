package models

import "gorm.io/gorm"

type Novel struct {
	gorm.Model
	Title       *string   `json:"title"`
	RawTitle    *string   `json:"raw_title"`
	Thumbnail   *string   `json:"thumbnail"`
	Author      *string   `json:"author"`
	Description *string   `json:"description"`
	URL         *string   `json:"url"`
	Chapters    []Chapter `json:"chapters"`
}

type Chapter struct {
	gorm.Model
	ID                uint    `gorm:"primarykey"`
	NovelID           uint    `gorm:"index:idx_novel_number,uniqueComposite"`
	Number            int     `gorm:"index:idx_novel_number,uniqueComposite"`
	Title             string  `json:"title"`
	TranslatedTitle   *string `json:"translated_title"`
	Content           *string `json:"content"`
	TranslatedContent *string `json:"translated_content"`
	TranslationStatus string  `json:"translation_status"`
	URL               string  `json:"url"`
}
