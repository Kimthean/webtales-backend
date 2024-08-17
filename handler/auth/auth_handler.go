package handler

import (
	"go-novel/models"
	"net/http"

	"go-novel/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

func (h *AuthHandler) SignUp(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	name := c.PostForm("username")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not hash password"})
		return
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Username:     name,
	}

	result := h.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Email already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Register success"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	var user models.User

	result := h.DB.Where("email = ?", email).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Email not found"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateJWT(user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
