package user

import (
	"go-novel/models"
	"go-novel/types"
	"go-novel/utils"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserHandler struct {
	DB *gorm.DB
}

// GetCurrentUser godoc
// @Summary Get current user
// @Description Get detailed information about the currently authenticated user
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /user/me [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	log.Println(userID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"email":        user.Email,
			"profileImage": user.ProfileImage,
			"role":         user.Role,
			"createdAt":    user.CreatedAt,
			"updatedAt":    user.UpdatedAt,
		},
	})
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update the profile information of the authenticated user
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body types.UpdateProfileRequest true "Profile update information"
// @Success 200 {object} map[string]string "Profile updated successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Failed to update profile"
// @Router /user/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var updateReq types.UpdateProfileRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if updateReq.Username != "" {
		user.Username = updateReq.Username
	}

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// UploadProfilePicture godoc
// @Summary Upload profile picture
// @Description Upload a new profile picture for the authenticated user
// @Tags user
// @Accept multipart/form-data
// @Produce json
// @Security ApiKeyAuth
// @Param profile_picture formData file true "Profile picture file"
// @Success 200 {object} map[string]string "Profile picture updated successfully"
// @Failure 400 {object} map[string]string "Invalid file type or size"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Failed to upload profile picture"
// @Router /user/profile-picture [post]
func (h *UserHandler) UploadProfilePicture(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	file, err := c.FormFile("profile_picture")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	if !isValidImageFile(file) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type or size"})
		return
	}

	tempFilePath := filepath.Join("tmp", file.Filename)
	if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	s3URL, err := utils.UploadFileToS3(tempFilePath, "profile-pictures", filepath.Ext(file.Filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload to S3"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.ProfileImage = s3URL
	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile picture"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile picture updated successfully", "url": s3URL})
}

func isValidImageFile(file *multipart.FileHeader) bool {
	if file.Size > 5*1024*1024 {
		return false
	}

	ext := filepath.Ext(file.Filename)
	validExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	return validExtensions[ext]
}

// ChangePassword godoc
// @Summary Change user password
// @Description Change the password for the authenticated user
// @Tags user
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer {token}"
// @Param request body types.ChangePasswordRequest true "Password change request"
// @Success 200 {object} map[string]string "Password changed successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /user/change-password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var changePasswordReq struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&changePasswordReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Provider == "google" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change password for Google Auth users"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(changePasswordReq.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(changePasswordReq.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash new password"})
		return
	}

	user.PasswordHash = string(hashedPassword)
	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// AddNovelToBookmark godoc
// @Summary Add novel to bookmarks
// @Description Add a novel to the authenticated user's bookmarks
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param novelID path int true "Novel ID"
// @Success 200 {object} map[string]string "Novel added to bookmarks successfully"
// @Failure 400 {object} map[string]string "Invalid novel ID"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 404 {object} map[string]string "User or novel not found"
// @Failure 409 {object} map[string]string "Novel already bookmarked"
// @Failure 500 {object} map[string]string "Failed to add novel to bookmarks"
// @Router /user/bookmark/{novelID} [post]
func (h *UserHandler) AddNovelToBookmark(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var novel models.Novel
	if err := h.DB.First(&novel, uint(novelID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
		return
	}

	var count int64
	h.DB.Model(&user).Where("bookmarks.novel_id = ?", novelID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Novel already bookmarked"})
		return
	}

	bookmark := models.Bookmark{
		UserID:  uint(userID.(uint)),
		NovelID: uint(novelID),
	}

	if err := h.DB.Create(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add novel to bookmarks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Novel added to bookmarks successfully"})
}

// GetUserBookmarks godoc
// @Summary Get user bookmarks
// @Description Get a paginated list of the authenticated user's bookmarked novels
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Invalid page number or page size"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Failed to fetch bookmarks"
// @Router /user/bookmarks [get]
func (h *UserHandler) GetUserBookmarks(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
		return
	}

	offset := (page - 1) * pageSize

	var totalBookmarks int64
	if err := h.DB.Table("bookmarks").
		Where("user_id = ?", userID).
		Count(&totalBookmarks).Error; err != nil {
		log.Printf("Error counting bookmarks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count bookmarks"})
		return
	}

	var bookmarks []struct {
		models.Novel
		TotalChaptersCount int       `json:"total_chapters_count"`
		BookmarkCreatedAt  time.Time `json:"bookmark_created_at"`
	}

	err = h.DB.Table("bookmarks").
		Select("novels.*, COUNT(chapters.id) as total_chapters_count, MAX(bookmarks.created_at) as bookmark_created_at").
		Joins("JOIN novels ON novels.id = bookmarks.novel_id").
		Joins("LEFT JOIN chapters ON chapters.novel_id = novels.id").
		Where("bookmarks.user_id = ? AND bookmarks.deleted_at IS NULL", userID).
		Group("novels.id").
		Order("MAX(bookmarks.created_at) DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&bookmarks).Error

	if err != nil {
		log.Printf("Error fetching bookmarks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookmarks"})
		return
	}

	totalPages := int(math.Ceil(float64(totalBookmarks) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"bookmarks":      bookmarks,
		"totalBookmarks": totalBookmarks,
		"currentPage":    page,
		"pageSize":       pageSize,
		"totalPages":     totalPages,
	})
}

// GetBookmark godoc
// @Summary Check if novel is bookmarked
// @Description Check if a specific novel is bookmarked by the authenticated user
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param novelID path int true "Novel ID"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} map[string]string "Invalid novel ID"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 500 {object} map[string]string "Failed to check bookmark status"
// @Router /user/bookmark/{novelID} [get]
func (h *UserHandler) GetBookmark(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	var count int64
	err = h.DB.Table("bookmarks").
		Where("user_id = ? AND novel_id = ?", userID, novelID).
		Count(&count).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check bookmark status"})
		return
	}

	isBookmarked := count > 0
	c.JSON(http.StatusOK, gin.H{"bookmarked": isBookmarked})
}

// RemoveNovelFromBookmark godoc
// @Summary Remove novel from bookmarks
// @Description Remove a novel from the authenticated user's bookmarks
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param novelID path int true "Novel ID"
// @Success 200 {object} map[string]string "Novel removed from bookmarks successfully"
// @Failure 400 {object} map[string]string "Invalid novel ID"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 500 {object} map[string]string "Failed to remove novel from bookmarks"
// @Router /user/bookmark/{novelID} [delete]
func (h *UserHandler) RemoveNovelFromBookmark(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	if err := h.DB.Unscoped().Where("user_id = ? AND novel_id = ?", userID, novelID).Delete(&models.Bookmark{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove novel from bookmarks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Novel removed from bookmarks successfully"})
}

// UpdateReadingProgress godoc
// @Summary Update reading progress
// @Description Update the reading progress for a specific novel and chapter
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param novelSlug path string true "Novel Slug"
// @Param chapterSlug path string true "Chapter Slug"
// @Success 200 {object} map[string]string "Reading progress updated successfully"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 404 {object} map[string]string "Novel or chapter not found"
// @Failure 500 {object} map[string]string "Failed to update reading progress"
// @Router /user/progress/{novelSlug}/{chapterSlug} [put]
func (h *UserHandler) UpdateReadingProgress(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	novelSlug := c.Param("novelSlug")
	chapterSlug := c.Param("chapterSlug")

	var novel models.Novel
	if err := h.DB.Where("slug = ?", novelSlug).First(&novel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch novel"})
		return
	}

	var chapter models.Chapter
	if err := h.DB.Where("novel_id = ? AND slug = ?", novel.ID, chapterSlug).First(&chapter).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chapter not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chapter"})
		return
	}

	var progress models.Progress
	if err := h.DB.Where("user_id = ? AND novel_id = ?", userID, novel.ID).First(&progress).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			progress = models.Progress{
				UserID:     uint(userID.(uint)),
				NovelID:    novel.ID,
				ChapterID:  chapter.ID,
				LastReadAt: time.Now(),
			}
			if err := h.DB.Create(&progress).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reading progress"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check reading progress"})
			return
		}
	} else {
		progress.ChapterID = chapter.ID
		progress.LastReadAt = time.Now()
		if err := h.DB.Save(&progress).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reading progress"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reading progress updated successfully"})
}

// GetReadingProgress godoc
// @Summary Get reading progress
// @Description Get the reading progress for a specific novel
// @Tags user
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param novelSlug path string true "Novel Slug"
// @Success 200 {object} map[string]interface{}
// @Router /user/progress/{novelSlug} [get]
func (h *UserHandler) GetReadingProgress(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	novelSlug := c.Param("novelSlug")

	var novel models.Novel
	if err := h.DB.Where("slug = ?", novelSlug).First(&novel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch novel"})
		return
	}

	var progress models.Progress
	if err := h.DB.Where("user_id = ? AND novel_id = ?", userID, novel.ID).First(&progress).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"progress": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve reading progress"})
		return
	}

	var chapter models.Chapter
	if err := h.DB.First(&chapter, progress.ChapterID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chapter details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"novel_slug":     novel.Slug,
		"chapter_slug":   chapter.Slug,
		"chapter_number": chapter.Number,
		"last_read_at":   progress.LastReadAt,
	})
}

// GetReadingHistory godoc
// @Summary Get user's reading history
// @Description Get a paginated list of novels and last read chapters for the authenticated user
// @Tags novels
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /novel/reading-history [get]
func (h *UserHandler) GetReadingHistory(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
		return
	}

	offset := (page - 1) * pageSize

	var totalHistory int64
	if err := h.DB.Table("progresses").
		Where("user_id = ?", userID).
		Count(&totalHistory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count reading history"})
		return
	}

	var readingHistory []struct {
		models.Novel
		LastReadAt    time.Time `json:"last_read_at"`
		ChapterID     uint      `json:"chapter_id"`
		ChapterTitle  string    `json:"chapter_title"`
		ChapterSlug   string    `json:"chapter_slug"`
		ChapterNumber int       `json:"chapter_number"`
	}

	err = h.DB.Table("progresses").
		Select("novels.*, progresses.last_read_at, progresses.chapter_id, chapters.slug as chapter_slug, chapters.number as chapter_number, chapters.translated_title as chapter_title").
		Joins("JOIN novels ON novels.id = progresses.novel_id").
		Joins("JOIN chapters ON chapters.id = progresses.chapter_id").
		Where("progresses.user_id = ?", userID).
		Order("progresses.last_read_at DESC").
		Limit(pageSize).Offset(offset).
		Scan(&readingHistory).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reading history"})
		return
	}

	totalPages := int(math.Ceil(float64(totalHistory) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"reading_history": readingHistory,
		"total_history":   totalHistory,
		"current_page":    page,
		"page_size":       pageSize,
		"total_pages":     totalPages,
	})
}
