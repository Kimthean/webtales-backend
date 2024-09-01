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

	count := h.DB.Model(&user).Where("id = ?", novelID).Association("Bookmarks").Count()
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Novel already bookmarked"})
		return
	}

	if err := h.DB.Model(&user).Association("Bookmarks").Append(&novel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add novel to bookmarks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Novel added to bookmarks successfully"})
}

func (h *UserHandler) GetUserBookmarks(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Pagination parameters
	var page, pageSize int = 1, 10
	var err error

	if qp := c.Query("page"); qp != "" {
		page, err = strconv.Atoi(qp)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
			return
		}
	}
	if qp := c.Query("pageSize"); qp != "" {
		pageSize, err = strconv.Atoi(qp)
		if err != nil || pageSize < 1 || pageSize > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
			return
		}
	}

	offset := (page - 1) * pageSize

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var bookmarks []struct {
		models.Novel
	}

	// Get total count of bookmarks
	var totalBookmarks int64
	if err := h.DB.Table("user_novels").
		Where("user_novels.user_id = ?", user.ID).
		Count(&totalBookmarks).Error; err != nil {
		log.Printf("Error counting bookmarks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count bookmarks"})
		return
	}

	// Fetch paginated bookmarks
	err = h.DB.Table("user_novels").
		Select("novels.*").
		Joins("JOIN novels ON novels.id = user_novels.novel_id").
		Where("user_novels.user_id = ?", user.ID).
		Limit(pageSize).
		Offset(offset).
		Scan(&bookmarks).Error

	if err != nil {
		log.Printf("Error fetching bookmarks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookmarks"})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalBookmarks) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"bookmarks":      bookmarks,
		"totalBookmarks": totalBookmarks,
		"currentPage":    page,
		"pageSize":       pageSize,
		"totalPages":     totalPages,
	})
}



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
	err = h.DB.Table("user_novels").
		Where("user_id = ? AND novel_id = ?", userID, novelID).
		Count(&count).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check bookmark status"})
		return
	}

	isBookmarked := count > 0
	c.JSON(http.StatusOK, gin.H{"bookmarked": isBookmarked})
}

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

	if err := h.DB.Table("user_novels").
		Where("user_id = ? AND novel_id = ?", userID, novelID).
		Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove novel from bookmarks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Novel removed from bookmarks successfully"})
}

func (h *UserHandler) UpdateReadingProgress(c *gin.Context) {
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

	chapterID, err := strconv.ParseUint(c.Param("chapterID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chapter ID"})
		return
	}

	var progress models.Progress
	if err := h.DB.Where("user_id = ? AND novel_id = ?", userID, novelID).First(&progress).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			progress = models.Progress{
				UserID:     uint(userID.(uint)),
				NovelID:    uint(novelID),
				ChapterID:  uint(chapterID),
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

		progress.ChapterID = uint(chapterID)
		progress.LastReadAt = time.Now()
		if err := h.DB.Save(&progress).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reading progress"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reading progress updated successfully"})
}

func (h *UserHandler) GetReadingProgress(c *gin.Context) {
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

	var progress models.Progress
	if err := h.DB.Where("user_id = ? AND novel_id = ?", userID, novelID).First(&progress).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"progress": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve reading progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"novel_id":     progress.NovelID,
		"chapter_id":   progress.ChapterID,
		"last_read_at": progress.LastReadAt,
	})
}
