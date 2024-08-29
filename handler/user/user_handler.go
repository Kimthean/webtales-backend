package user

import (
	"go-novel/models"
	"go-novel/types"
	"go-novel/utils"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	DB *gorm.DB
}

// GetCurrentUser retrieves the current user's profile
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
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

// UpdateProfile updates the user's profile information
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

	// Update user fields
	if updateReq.Username != "" {
		user.Username = updateReq.Username
	}
	// Add more fields as needed

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// UploadProfilePicture handles the upload of a new profile picture
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

	// Check file type and size
	if !isValidImageFile(file) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type or size"})
		return
	}

	// Save file temporarily
	tempFilePath := filepath.Join("tmp", file.Filename)
	if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Upload to S3
	s3URL, err := utils.UploadFileToS3(tempFilePath, "profile-pictures", filepath.Ext(file.Filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload to S3"})
		return
	}

	// Update user's profile picture URL in the database
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

// Helper function to validate image file
func isValidImageFile(file *multipart.FileHeader) bool {
	// Check file size (e.g., max 5MB)
	if file.Size > 5*1024*1024 {
		return false
	}

	// Check file type
	ext := filepath.Ext(file.Filename)
	validExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	return validExtensions[ext]
}
