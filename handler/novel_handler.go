package handlers

import (
	"go-novel/models"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NovelHandler struct {
	DB *gorm.DB
}

type ChapterResponse struct {
	ID              uint      `json:"ID"`
	Number          int       `json:"Number"`
	UpdatedAt       time.Time `json:"UpdatedAt"`
	TranslatedTitle string    `json:"translated_title"`
}

type NovelResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	RawTitle    string    `json:"raw_title"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	Thumbnail   string    `json:"thumbnail"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *NovelHandler) GetNovel(c *gin.Context) {
	id := c.Param("id")
	var novelResponse NovelResponse
	var novel models.Novel
	if err := h.DB.Table("novels").Select("id, title, raw_title, author, description, thumbnail, updated_at").
		Where("id = ?", id).First(&novel).
		Scan(&novelResponse).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, novelResponse)
}

func (h *NovelHandler) GetNovels(c *gin.Context) {
	var novelResponses []struct {
		models.Novel
		LastChapterTitle   string `json:"last_chapter_title"`
		LastChapterNumber  int    `json:"last_chapter_number"`
		TotalChaptersCount int    `json:"total_chapters_count"`
	}

	maxChapterIDSubquery := h.DB.Table("chapters").
		Select("MAX(id) as id, novel_id").
		Group("novel_id")

	chapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as total_chapters_count, novel_id").
		Group("novel_id")

	if err := h.DB.Table("novels").
		Select("novels.*, c.number as last_chapter_number, c.translated_title as last_chapter_title, cc.total_chapters_count").
		Joins("LEFT JOIN (?) as mc ON mc.novel_id = novels.id", maxChapterIDSubquery).
		Joins("LEFT JOIN chapters as c ON mc.id = c.id").
		Joins("LEFT JOIN (?) as cc ON cc.novel_id = novels.id", chapterCountSubquery).
		Where("novels.deleted_at IS NULL").
		Order("novels.updated_at DESC").
		Scan(&novelResponses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, novelResponses)
}

func (h *NovelHandler) GetLatestNovels(c *gin.Context) {
	var novels []models.Novel
	h.DB.Order("created_at DESC").Find(&novels)
	c.JSON(http.StatusOK, novels)
}

func (h *NovelHandler) GetNovelChapters(c *gin.Context) {
	id := c.Param("id")

	var chapterResponses []ChapterResponse
	if err := h.DB.Table("chapters").
		Select("id, number, updated_at, translated_title, translation_status").
		Where("novel_id = ?", id).
		Order("number ASC").
		Scan(&chapterResponses).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chapterResponses)
}

func (h *NovelHandler) GetNovelChaptersWithPage(c *gin.Context) {
	id := c.Param("id")

	var page, pageSize int = 1, 20
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

	var chapterResponses []ChapterResponse
	var totalChapters int64

	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Count(&totalChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Table("chapters").
		Select("id, number, updated_at, translated_title, translation_status").
		Where("novel_id = ?", id).
		Order("number ASC").
		Limit(pageSize).
		Offset(offset).
		Scan(&chapterResponses).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"chapters":      chapterResponses,
		"totalChapters": totalChapters,
		"currentPage":   page,
		"pageSize":      pageSize,
		"totalPages":    int(math.Ceil(float64(totalChapters) / float64(pageSize))),
	}

	c.JSON(http.StatusOK, response)
}

func (h *NovelHandler) GetChapterByID(c *gin.Context) {
	id := c.Param("id")
	chapterNumber := c.Param("number")

	var response struct {
		ID                uint      `json:"id"`
		Number            int       `json:"number"`
		UpdatedAt         time.Time `json:"updated_at"`
		TranslatedTitle   string    `json:"translated_title"`
		TranslationStatus string    `json:"translation_status"`
		TranslatedContent string    `json:"translated_content"`
		NovelID           string    `json:"novel_id"`
		NovelTitle        string    `json:"novel_title"`
	}

	if err := h.DB.Table("chapters").
		Select("chapters.id, chapters.number, chapters.updated_at, chapters.translated_title, chapters.translation_status, chapters.translated_content, novels.title as novel_title, novels.id as novel_id").
		Joins("join novels on novels.id = chapters.novel_id").
		Where("chapters.novel_id = ? AND chapters.number = ?", id, chapterNumber).
		First(&response).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chapter not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *NovelHandler) GetNovelTranslationStatus(c *gin.Context) {
	id := c.Param("id")
	var totalChapters, translatedChapters int64
	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Count(&totalChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ? AND translation_status = ?", id, "completed").Count(&translatedChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := "in_progress"
	if translatedChapters == totalChapters {
		status = "completed"
	}

	c.JSON(http.StatusOK, gin.H{
		"total_chapters":      totalChapters,
		"translated_chapters": translatedChapters,
		"status":              status,
	})
}

func (h *NovelHandler) DeleteNovelByID(c *gin.Context) {
	id := c.Param("id")

	tx := h.DB.Begin()

	if err := tx.Unscoped().Where("novel_id = ?", id).Delete(&models.Chapter{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting associated chapters"})
		return
	}

	if err := tx.Unscoped().Where("id = ?", id).Delete(&models.Novel{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting novel"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error committing transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Novel and associated chapters deleted permanently"})
}

func (h *NovelHandler) SearchNovels(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	var novels []struct {
		ID                 uint      `json:"id"`
		Title              string    `json:"title"`
		Author             string    `json:"author"`
		Description        string    `json:"description"`
		Thumbnail          string    `json:"thumbnail"`
		UpdatedAt          time.Time `json:"updated_at"`
		TotalChaptersCount int       `json:"total_chapters_count"`
		Status             struct {
			Status string `json:"status"`
		} `json:"status"`
	}

	if err := h.DB.Table("novels").
		Select("novels.id, novels.title, novels.author, novels.description, novels.thumbnail, novels.updated_at, "+
			"COUNT(chapters.id) as total_chapters_count, "+
			"CASE WHEN COUNT(chapters.id) = SUM(CASE WHEN chapters.translation_status = 'completed' THEN 1 ELSE 0 END) THEN 'completed' ELSE 'in_progress' END as status").
		Joins("LEFT JOIN chapters ON chapters.novel_id = novels.id").
		Where("novels.title ILIKE ? OR novels.author ILIKE ? OR novels.description ILIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%").
		Group("novels.id").
		Scan(&novels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error searching novels"})
		return
	}

	c.JSON(http.StatusOK, novels)
}

func (h *NovelHandler) GetPaginatedNovels(c *gin.Context) {
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

	var novelResponses []struct {
		models.Novel
		LastChapterTitle   string `json:"last_chapter_title"`
		LastChapterNumber  int    `json:"last_chapter_number"`
		TotalChaptersCount int    `json:"total_chapters_count"`
	}

	var totalNovels int64

	maxChapterIDSubquery := h.DB.Table("chapters").
		Select("MAX(id) as id, novel_id").
		Group("novel_id")

	chapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as total_chapters_count, novel_id").
		Group("novel_id")

	query := h.DB.Table("novels").
		Select("novels.*, c.number as last_chapter_number, c.translated_title as last_chapter_title, cc.total_chapters_count").
		Joins("LEFT JOIN (?) as mc ON mc.novel_id = novels.id", maxChapterIDSubquery).
		Joins("LEFT JOIN chapters as c ON mc.id = c.id").
		Joins("LEFT JOIN (?) as cc ON cc.novel_id = novels.id", chapterCountSubquery).
		Where("novels.deleted_at IS NULL").
		Order("novels.updated_at DESC")

	if err := query.Count(&totalNovels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error counting novels"})
		return
	}

	if err := query.Limit(pageSize).Offset(offset).Scan(&novelResponses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching novels"})
		return
	}

	response := gin.H{
		"novels":      novelResponses,
		"totalNovels": totalNovels,
		"currentPage": page,
		"pageSize":    pageSize,
		"totalPages":  int(math.Ceil(float64(totalNovels) / float64(pageSize))),
	}

	c.JSON(http.StatusOK, response)
}
