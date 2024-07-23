package handlers

import (
	"go-novel/models"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
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

func (h *NovelHandler) GetNovel(c echo.Context) error {

	id := c.Param("id")
	var novelResponse NovelResponse
	var novel models.Novel
	if err := h.DB.Table("novels").Select("id, title, raw_title, author, description, thumbnail, updated_at").
		Where("id = ?", id).First(&novel).
		Scan(&novelResponse).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Novel not found")
		}
		return err
	}

	return c.JSON(http.StatusOK, novelResponse)
}

func (h *NovelHandler) GetNovels(c echo.Context) error {
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
		return err
	}

	return c.JSON(http.StatusOK, novelResponses)
}

func (h *NovelHandler) GetLatestNovels(c echo.Context) error {
	var novels []models.Novel
	h.DB.Order("created_at DESC").Find(&novels)
	return c.JSON(http.StatusOK, novels)
}

func (h *NovelHandler) GetNovelChapters(c echo.Context) error {
	id := c.Param("id")

	var chapterResponses []ChapterResponse
	if err := h.DB.Table("chapters").
		Select("id, number, updated_at, translated_title, translation_status").
		Where("novel_id = ?", id).
		Order("number ASC").
		Scan(&chapterResponses).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Novel not found")
		}
		return err
	}

	return c.JSON(http.StatusOK, chapterResponses)
}

func (h *NovelHandler) GetNovelChaptersWithPage(c echo.Context) error {
	id := c.Param("id")

	var page, pageSize int = 1, 20
	var err error

	if qp := c.QueryParam("page"); qp != "" {
		page, err = strconv.Atoi(qp)
		if err != nil || page < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid page number")
		}
	}
	if qp := c.QueryParam("pageSize"); qp != "" {
		pageSize, err = strconv.Atoi(qp)
		if err != nil || pageSize < 1 || pageSize > 100 { // Added upper limit
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid page size")
		}
	}

	offset := (page - 1) * pageSize

	var chapterResponses []ChapterResponse
	var totalChapters int64

	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Count(&totalChapters).Error; err != nil {
		return err
	}

	if err := h.DB.Table("chapters").
		Select("id, number, updated_at, translated_title, translation_status").
		Where("novel_id = ?", id).
		Order("number ASC").
		Limit(pageSize).
		Offset(offset).
		Scan(&chapterResponses).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Novel not found")
		}
		return err
	}

	response := map[string]interface{}{
		"chapters":      chapterResponses,
		"totalChapters": totalChapters,
		"currentPage":   page,
		"pageSize":      pageSize,
		"totalPages":    int(math.Ceil(float64(totalChapters) / float64(pageSize))),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *NovelHandler) GetChapterByID(c echo.Context) error {
	novelID := c.Param("novel_id")
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
		Where("chapters.novel_id = ? AND chapters.number = ?", novelID, chapterNumber).
		First(&response).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Chapter not found")
		}
		return err
	}

	return c.JSON(http.StatusOK, response)
}

func (h *NovelHandler) GetNovelTranslationStatus(c echo.Context) error {
	id := c.Param("id")
	var totalChapters, translatedChapters int64
	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Count(&totalChapters).Error; err != nil {
		return err
	}
	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ? AND translation_status = ?", id, "completed").Count(&translatedChapters).Error; err != nil {
		return err
	}

	status := "in_progress"
	if translatedChapters == totalChapters {
		status = "completed"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total_chapters":      totalChapters,
		"translated_chapters": translatedChapters,
		"status":              status,
	})
}

func (h *NovelHandler) DeleteNovelByID(c echo.Context) error {
	id := c.Param("id")

	tx := h.DB.Begin()

	// Hard delete associated chapters first
	if err := tx.Unscoped().Where("novel_id = ?", id).Delete(&models.Chapter{}).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting associated chapters")
	}

	// Hard delete the novel by using Unscoped() to bypass soft delete
	if err := tx.Unscoped().Where("id = ?", id).Delete(&models.Novel{}).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting novel")
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error committing transaction")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Novel and associated chapters deleted permanently"})
}
