package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username     string     `json:"username"`
	ProfileImage string     `json:"profile_image" gorm:"https://webtales.sgp1.digitaloceanspaces.com/cover/loading.jpg"`
	PasswordHash string     `json:"-"`
	Email        string     `json:"email" gorm:"uniqueIndex"`
	Role         string     `gorm:"default:user"`
	Provider     string     `gorm:"default:credential"`
	Bookmarks    []Novel    `gorm:"many2many:user_novels;" json:"bookmarks,omitempty"`
	Progress     []Progress `json:"progress,omitempty"`
}

type Progress struct {
	gorm.Model
	UserID     uint      `gorm:"index"` // Foreign key referencing User
	NovelID    uint      `gorm:"index"` // Foreign key referencing Novel
	ChapterID  uint      `gorm:"index"` // Foreign key referencing Chapter
	LastReadAt time.Time `gorm:"index"` // Timestamp of the last read
}
