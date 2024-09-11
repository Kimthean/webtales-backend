package handler

import (
	"go-novel/models"
	"net/http"

	"go-novel/types"
	"go-novel/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

type GoogleCallbackRequest struct {
	Code     string `json:"code"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
	GoogleID string `json:"googleId"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// GoogleCallback godoc
// @Summary Handle Google OAuth callback
// @Description Process Google OAuth callback and create or update user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body GoogleCallbackRequest true "Google callback data"
// @Router /auth/google/callback [post]
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	var req GoogleCallbackRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var dbUser models.User
	result := h.DB.Where("email = ?", req.Email).First(&dbUser)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			dbUser = models.User{
				Email:        req.Email,
				Username:     req.Name,
				ProfileImage: req.Picture,
				Role:         "user",
				Provider:     "google",
			}
			if err := h.DB.Create(&dbUser).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	} else {
		if err := h.DB.Save(&dbUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}
	accessToken, err := utils.GenerateAccessToken(dbUser.Username, dbUser.Role, dbUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
		return
	}

	refreshToken, err := utils.GenerateRefreshToken(dbUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": gin.H{
			"id":           dbUser.ID,
			"username":     dbUser.Username,
			"profileImage": dbUser.ProfileImage,
			"email":        dbUser.Email,
			"role":         dbUser.Role,
			"provider":     dbUser.Provider,
			"createdAt":    dbUser.CreatedAt,
			"updatedAt":    dbUser.UpdatedAt,
		},
	})
}

// SignUp godoc
// @Summary Register a new user
// @Description Create a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body types.SignupRequest true "User signup information"
// @Router /auth/signup [post]
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req types.SignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not hash password"})
		return
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Username:     req.Username,
		ProfileImage: "https://webtales.sgp1.digitaloceanspaces.com/cover/loading.jpg",
	}

	result := h.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Email already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Register success"})
}

// Login godoc
// @Summary Authenticate a user
// @Description Authenticate a user and return a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body types.LoginRequest true "User login credentials"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req types.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user models.User

	result := h.DB.Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Email not found"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid credentials"})
		return
	}

	accessToken, err := utils.GenerateAccessToken(user.Username, user.Role, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
		return
	}

	refreshToken, err := utils.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"profileImage": user.ProfileImage,
			"email":        user.Email,
			"role":         user.Role,
			"provider":     user.Provider,
			"createdAt":    user.CreatedAt,
			"updatedAt":    user.UpdatedAt,
		},
	})
}

// Refresh godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	claims, err := utils.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	userID := uint(claims["id"].(float64))

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	accessToken, err := utils.GenerateAccessToken(user.Username, user.Role, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
		return
	}

	refreshToken, err := utils.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

// GetAllUsers godoc
// @Summary Get all users
// @Description Retrieve a list of all users (admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Router /admin/users [get]
func (h *AuthHandler) GetAllUsers(c *gin.Context) {
	var users []models.User
	h.DB.Find(&users)
	c.JSON(http.StatusOK, gin.H{"data": users})
}
