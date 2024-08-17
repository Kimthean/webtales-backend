package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username     string   `json:"username"`
	PasswordHash string   `json:"-"`
	Email        string   `json:"email"`
	Role         string `gorm:"default:user"` // Default role is 'user'
	Bookmarks    []Novel  `gorm:"many2many:user_novels;" json:"bookmarks,omitempty"`
	Progress     []Progress `json:"progress,omitempty"` // Progress of the user
}


type Progress struct {
	gorm.Model
	UserID     uint      `gorm:"index"` // Foreign key referencing User
	NovelID    uint      `gorm:"index"` // Foreign key referencing Novel
	ChapterID  uint      `gorm:"index"` // Foreign key referencing Chapter
	LastReadAt time.Time `gorm:"index"`  // Timestamp of the last read
}
