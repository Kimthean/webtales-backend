package handlers

import (
	"go-novel/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GenreHandler struct {
	DB *gorm.DB
}

func (h *GenreHandler) GetGenres(c *gin.Context) {
	var genres []models.Genre
	if err := h.DB.Find(&genres).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch genres"})
		return
	}

	c.JSON(http.StatusOK, genres)
}

func (h *GenreHandler) CreateGenre(c *gin.Context) {
	var input struct {
		NameChinese string `json:"name_chinese" binding:"required"`
		NamePinyin  string `json:"name_pinyin" binding:"required"`
		NameEnglish string `json:"name_english" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	genre := models.Genre{
		NameChinese: input.NameChinese,
		NamePinyin:  input.NamePinyin,
		NameEnglish: input.NameEnglish,
	}

	if err := h.DB.Create(&genre).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create genre"})
		return
	}

	c.JSON(http.StatusCreated, genre)
}

func (h *GenreHandler) AddGenreToNovel(c *gin.Context) {
	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	genreID, err := strconv.ParseUint(c.Param("genreID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid genre ID"})
		return
	}

	var novel models.Novel
	if err := h.DB.First(&novel, novelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
		return
	}

	var genre models.Genre
	if err := h.DB.First(&genre, genreID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	if err := h.DB.Model(&novel).Association("Genres").Append(&genre); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add genre to novel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genre added to novel successfully"})
}

func (h *GenreHandler) DeleteGenreFromNovel(c *gin.Context) {
	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	genreID, err := strconv.ParseUint(c.Param("genreID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid genre ID"})
		return
	}

	var novel models.Novel
	if err := h.DB.First(&novel, novelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
		return
	}

	var genre models.Genre
	if err := h.DB.First(&genre, genreID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	if err := h.DB.Model(&novel).Association("Genres").Delete(&genre); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete genre from novel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genre deleted from novel successfully"})
}

func (h *GenreHandler) GetNovelGenres(c *gin.Context) {
	novelID, err := strconv.ParseUint(c.Param("novelID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid novel ID"})
		return
	}

	var novel models.Novel
	if err := h.DB.Preload("Genres").First(&novel, novelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
		return
	}

	c.JSON(http.StatusOK, novel.Genres)
}

func (h *GenreHandler) DeleteGenre(c *gin.Context) {
	genreID, err := strconv.ParseUint(c.Param("genreID"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid genre ID"})
		return
	}

	result := h.DB.Delete(&models.Genre{}, genreID)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete genre"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genre deleted successfully"})
}

func (h *GenreHandler) PopulateGenres(c *gin.Context) {
	genres := []models.Genre{
		{NameChinese: "仙侠", NamePinyin: "Xianxia", NameEnglish: "Immortal Heroes"},
		{NameChinese: "玄幻", NamePinyin: "Xuanhuan", NameEnglish: "Fantasy"},
		// ... add all other genres here
	}

	for _, genre := range genres {
		if err := h.DB.Create(&genre).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to populate genres"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Genres populated successfully"})
}
