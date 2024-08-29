package handler

import (
	"go-novel/models"
	"net/http"

	"go-novel/types"
	"go-novel/utils"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

func InitializeGothProviders(clientID, clientSecret, callbackURL string) {
	goth.UseProviders(
		google.New(clientID, clientSecret, callbackURL),
	)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Picture  string `json:"picture"`
		GoogleID string `json:"googleId"`
	}

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

	token, err := utils.GenerateJWT(dbUser.Username, dbUser.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":           dbUser.ID,
			"username":     dbUser.Username,
			"profileImage": dbUser.ProfileImage,
			"email":        dbUser.Email,
			"role":         dbUser.Role,
			"createdAt":    dbUser.CreatedAt,
			"updatedAt":    dbUser.UpdatedAt,
		},
	})
}

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
	}

	result := h.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Email already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Register success"})
}

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

	token, err := utils.GenerateJWT(user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"profileImage": user.ProfileImage,
			"email":        user.Email,
			"role":         user.Role,
			"createdAt":    user.CreatedAt,
			"updatedAt":    user.UpdatedAt,
		},
	})
}

func (h *AuthHandler) GetAllUsers(c *gin.Context) {
	var users []models.User
	h.DB.Find(&users)
	c.JSON(http.StatusOK, gin.H{"data": users})
}
