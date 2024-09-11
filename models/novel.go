package models

import (
	"time"

	"gorm.io/gorm"
)

type Novel struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`

	Title       *string   `json:"title"`
	Slug        *string   `json:"slug"`
	RawTitle    *string   `json:"raw_title"`
	Thumbnail   *string   `json:"thumbnail"`
	Author      *string   `json:"author"`
	Description *string   `json:"description"`
	URL         *string   `json:"url"`
	Chapters    []Chapter `json:"chapters"`
	EpubURL     *string   `json:"epub_url"`
	Tags        []*Tag    `gorm:"many2many:novel_tags;" json:"tags"`
	Genres      []*Genre  `gorm:"many2many:novel_genres;" json:"genres"`
}

type Chapter struct {
	gorm.Model
	ID                uint    `gorm:"primarykey"`
	NovelID           uint    `gorm:"index:idx_novel_number,uniqueComposite"`
	Number            int     `gorm:"index:idx_novel_number,uniqueComposite"`
	Title             string  `json:"title"`
	Slug              string  `json:"slug"`
	TranslatedTitle   *string `json:"translated_title"`
	Content           *string `json:"content"`
	TranslatedContent *string `json:"translated_content"`
	TranslationStatus string  `json:"translation_status"`
	URL               string  `json:"url"`
}

type Tag struct {
	gorm.Model
	Name string `json:"name"`
}

type Genre struct {
	gorm.Model
	NameChinese string `json:"name_chinese"`
	NamePinyin  string `json:"name_pinyin"`
	NameEnglish string `json:"name_english"`
}
