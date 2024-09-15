package handlers

import (
	"fmt"
	"go-novel/lib"
	"go-novel/models"
	"go-novel/utils"
	"go-novel/worker"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NovelHandler struct {
	DB     *gorm.DB
	Worker *worker.Worker
}

type ChapterResponse struct {
	ID              uint      `json:"ID"`
	Slug            string    `json:"slug"`
	Number          int       `json:"number"`
	UpdatedAt       time.Time `json:"updated_at"`
	TranslatedTitle string    `json:"translated_title"`
}

type PaginatedChapterResponse struct {
	Chapters      []ChapterResponse `json:"chapters"`
	TotalChapters int64             `json:"totalChapters"`
	CurrentPage   int               `json:"currentPage"`
	PageSize      int               `json:"pageSize"`
	TotalPages    int               `json:"totalPages"`
}

type NovelResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	RawTitle    string    `json:"raw_title"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	EpubURL     string    `json:"epub_url"`
	Thumbnail   string    `json:"thumbnail"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NovelUpdateResponse struct {
	ID                 uint      `json:"id"`
	Title              *string   `json:"title"`
	RawTitle           *string   `json:"raw_title"`
	Slug               *string   `json:"slug"`
	Author             *string   `json:"author"`
	Description        *string   `json:"description"`
	Thumbnail          *string   `json:"thumbnail"`
	EpubURL            *string   `json:"epub_url"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	LastChapterDate    time.Time `json:"last_chapter_date"`
	TotalChaptersCount int       `json:"total_chapters_count"`
}

// CrawlNovel godoc
// @Summary Crawl a novel
// @Description Crawl a novel from the given URL and optionally use AI for translation
// @Tags admin
// @Accept json
// @Produce json
// @Param url query string true "URL to crawl"
// @Param ai query bool false "Use AI for translation" default(false)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} string "Bad request"
// @Failure 500 {object} string "Internal server error"
// @Router /admin/crawl [post]
func (h *NovelHandler) CrawlNovel(c *gin.Context) {
	url := c.Query("url")

	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	if err := h.Worker.EnqueueNovel(url); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to enqueue novel: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Novel crawling initiated",
		"url":     url,
	})
}

// GetNovel godoc
// @Summary Get a novel by slug
// @Description Get detailed information about a novel
// @Tags novels
// @Accept json
// @Produce json
// @Param novelSlug path string true "Novel Slug"
// @Success 200 {object} NovelResponse
// @Failure 404 {object} string "Novel not found"
// @Failure 500 {object} string "Internal server error"
// @Router /novel/{novelSlug} [get]
func (h *NovelHandler) GetNovel(c *gin.Context) {
	slug := c.Param("novelSlug")

	var novel models.Novel
	if err := h.DB.Preload("Tags").Preload("Genres").Where("slug = ?", slug).First(&novel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Println("Novel not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		log.Printf("Error fetching novel: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var firstChapter models.Chapter
	if err := h.DB.Where("novel_id = ?", novel.ID).Order("number ASC").First(&firstChapter).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Println("No chapters found for the novel")
			c.JSON(http.StatusOK, gin.H{
				"id":                   novel.ID,
				"title":                novel.Title,
				"raw_title":            novel.RawTitle,
				"author":               novel.Author,
				"description":          novel.Description,
				"thumbnail":            novel.Thumbnail,
				"epub_url":             novel.EpubURL,
				"updated_at":           novel.UpdatedAt,
				"created_at":           novel.CreatedAt,
				"tags":                 novel.Tags,
				"genres":               novel.Genres,
				"first_chapter_slug":   "",
				"first_chapter_number": 0,
			})
			return
		}
		log.Printf("Error fetching first chapter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   novel.ID,
		"title":                novel.Title,
		"raw_title":            novel.RawTitle,
		"author":               novel.Author,
		"description":          novel.Description,
		"thumbnail":            novel.Thumbnail,
		"epub_url":             novel.EpubURL,
		"updated_at":           novel.UpdatedAt,
		"created_at":           novel.CreatedAt,
		"tags":                 novel.Tags,
		"genres":               novel.Genres,
		"first_chapter_slug":   firstChapter.Slug,
		"first_chapter_number": firstChapter.Number,
	})
}

// GetNovels godoc
// @Summary Get all novels
// @Description Get a list of all novels with their latest chapter information
// @Tags novels
// @Accept json
// @Produce json
// @Success 200 {array} NovelResponse
// @Failure 500 {object} string "Internal server error"
// @Router /novel/all [get]
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

// GetLatestNovels godoc
// @Summary Get latest novels
// @Description Get a list of the 6 most recently added novels
// @Tags novels
// @Accept json
// @Produce json
// @Success 200 {array} NovelUpdateResponse
// @Failure 500 {object} string "Internal server error"
// @Router /novel/latest [get]
func (h *NovelHandler) GetLatestNovels(c *gin.Context) {
	var novelResponses []NovelUpdateResponse

	chapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as total_chapters_count, novel_id").
		Group("novel_id")

	if err := h.DB.Table("novels").
		Select("novels.id, novels.title, novels.slug, novels.raw_title, novels.description, novels.thumbnail, novels.author, novels.updated_at, novels.created_at, novels.epub_url, COALESCE(cc.total_chapters_count, 0) as total_chapters_count").
		Joins("LEFT JOIN (?) as cc ON cc.novel_id = novels.id", chapterCountSubquery).
		Where("novels.deleted_at IS NULL").
		Order("novels.created_at DESC").
		Limit(6).
		Scan(&novelResponses).Error; err != nil {
		log.Printf("Error fetching latest novels: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest novels"})
		return
	}
	c.JSON(http.StatusOK, novelResponses)
}

// GetLatestUpdate godoc
// @Summary Get latest novel updates
// @Description Get a list of the 6 most recently updated novels
// @Tags novels
// @Accept json
// @Produce json
// @Success 200 {array} NovelUpdateResponse
// @Failure 500 {object} string "Internal server error"
// @Router /novel/latest-update [get]
func (h *NovelHandler) GetLatestUpdate(c *gin.Context) {
	var novelUpdates []NovelUpdateResponse

	latestChapterSubquery := h.DB.Table("chapters").
		Select("novel_id, MAX(updated_at) as last_chapter_date").
		Group("novel_id")

	chapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as total_chapters_count, novel_id").
		Group("novel_id")

	if err := h.DB.Table("novels").
		Select("novels.id, novels.title, novels.slug, novels.raw_title, novels.description, novels.thumbnail, novels.author, novels.updated_at, novels.created_at, novels.epub_url, lc.last_chapter_date, COALESCE(cc.total_chapters_count, 0) as total_chapters_count").
		Joins("JOIN (?) as lc ON lc.novel_id = novels.id", latestChapterSubquery).
		Joins("LEFT JOIN (?) as cc ON cc.novel_id = novels.id", chapterCountSubquery).
		Order("lc.last_chapter_date DESC").
		Limit(6).
		Scan(&novelUpdates).Error; err != nil {
		log.Printf("Error fetching latest updates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest updates"})
		return
	}
	c.JSON(http.StatusOK, novelUpdates)
}

// GetNovelChaptersWithPage godoc
// @Summary Get paginated novel chapters
// @Description Get a paginated list of chapters for a specific novel
// @Tags novels
// @Accept json
// @Produce json
// @Param novelSlug path string true "Novel Slug"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Failure 400 {object} string "Invalid page number or page size"
// @Failure 404 {object} string "Novel not found"
// @Failure 500 {object} string "Internal server error"
// @Router /novel/{novelSlug}/paginate-chapters [get]
func (h *NovelHandler) GetNovelChaptersWithPage(c *gin.Context) {
	novelSlug := c.Param("novelSlug")

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

	var novelID int
	if err := h.DB.Table("novels").Select("id").Where("slug = ?", novelSlug).Scan(&novelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var chapterResponses []ChapterResponse
	var totalChapters int64

	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).Count(&totalChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Table("chapters").
		Select("id, number, slug, updated_at, translated_title, translation_status").
		Where("novel_id = ?", novelID).
		Order("number ASC").
		Limit(pageSize).
		Offset(offset).
		Scan(&chapterResponses).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chapters not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := PaginatedChapterResponse{
		Chapters:      chapterResponses,
		TotalChapters: totalChapters,
		CurrentPage:   page,
		PageSize:      pageSize,
		TotalPages:    int(math.Ceil(float64(totalChapters) / float64(pageSize))),
	}

	c.JSON(http.StatusOK, response)
}

// GetChapterBySlug godoc
// @Summary Get a specific chapter
// @Description Get details of a specific chapter by novel slug and chapter slug
// @Tags novels
// @Accept json
// @Produce json
// @Param novelSlug path string true "Novel Slug"
// @Param chapterSlug path string true "Chapter Slug"
// @Success 200 {object} ChapterResponse
// @Failure 404 {object} string "Chapter not found"
// @Failure 500 {object} string "Internal server error"
// @Router /novel/{novelSlug}/chapter/{chapterSlug} [get]
func (h *NovelHandler) GetChapterBySlug(c *gin.Context) {
	novelSlug := c.Param("novelSlug")
	chapterSlug := c.Param("chapterSlug")

	var response struct {
		ID                uint      `json:"id"`
		Number            int       `json:"number"`
		Slug              string    `json:"slug"`
		UpdatedAt         time.Time `json:"updated_at"`
		TranslatedTitle   string    `json:"translated_title"`
		TranslationStatus string    `json:"translation_status"`
		TranslatedContent string    `json:"translated_content"`
		NovelID           string    `json:"novel_id"`
		NovelTitle        string    `json:"novel_title"`
		NextChapterSlug   string    `json:"next_chapter_slug,omitempty"`
		PrevChapterSlug   string    `json:"prev_chapter_slug,omitempty"`
	}

	if err := h.DB.Table("chapters").
		Select("chapters.id, chapters.number, chapters.slug, chapters.updated_at, chapters.translated_title, chapters.translation_status, chapters.translated_content, novels.title as novel_title, novels.id as novel_id, next_chapter.slug as next_chapter_slug, prev_chapter.slug as prev_chapter_slug").
		Joins("join novels on novels.id = chapters.novel_id").
		Joins("left join chapters as next_chapter on next_chapter.novel_id = chapters.novel_id and next_chapter.number = chapters.number + 1").
		Joins("left join chapters as prev_chapter on prev_chapter.novel_id = chapters.novel_id and prev_chapter.number = chapters.number - 1").
		Where("novels.slug = ? AND chapters.slug = ?", novelSlug, chapterSlug).
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

// GetNovel godoc
// @Summary Get a novel by slug
// @Description Get detailed information about a novel
// @Tags novels
// @Accept json
// @Produce json
// @Param novelSlug path string true "Novel Slug"
// @Success 200 {object} NovelResponse
// @Failure 404 {object} map[string]string "Novel not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /novel/{novelSlug} [get]
func (h *NovelHandler) GetNovelTranslationStatus(c *gin.Context) {
	novelSlug := c.Param("novelSlug")
	var novel models.Novel
	var totalChapters, translatedChapters int64

	if err := h.DB.Model(&models.Novel{}).Where("slug = ?", novelSlug).First(&novel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Novel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ?", novel.ID).Count(&totalChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Model(&models.Chapter{}).Where("novel_id = ? AND translation_status = ?", novel.ID, "completed").Count(&translatedChapters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := "in_progress"
	if translatedChapters == totalChapters {
		status = "completed"
	}

	// Return the translation status
	c.JSON(http.StatusOK, gin.H{
		"novel_id":            novel.ID,
		"total_chapters":      totalChapters,
		"translated_chapters": translatedChapters,
		"status":              status,
	})
}

// DeleteNovelByID godoc
// @Summary Delete a novel
// @Description Delete a novel and its associated chapters by ID
// @Tags admin
// @Accept json
// @Produce json
// @Param id path int true "Novel ID"
// @Success 200 {object} map[string]string "Novel and associated chapters deleted"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /admin/{id} [delete]
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

// SearchNovels godoc
// @Summary Search novels
// @Description Search for novels by title, author, or description
// @Tags novels
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Success 200 {array} NovelResponse
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /novel/search [get]
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

// GetPaginatedNovels godoc
// @Summary Get paginated novels
// @Description Get a paginated list of all novels
// @Tags novels
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} string "Invalid page number or page size"
// @Failure 500 {object} string "Internal server error"
// @Router /novel [get]
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
		TranslatedChapters int    `json:"translated_chapters"`
		TranslationStatus  string `json:"translation_status"`
	}

	var totalNovels int64

	maxChapterIDSubquery := h.DB.Table("chapters").
		Select("MAX(id) as id, novel_id").
		Group("novel_id")

	chapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as total_chapters_count, novel_id").
		Group("novel_id")

	translatedChapterCountSubquery := h.DB.Table("chapters").
		Select("COUNT(id) as translated_chapters, novel_id").
		Where("translation_status = ?", "completed").
		Group("novel_id")

	query := h.DB.Table("novels").
		Select("novels.*, c.number as last_chapter_number, c.translated_title as last_chapter_title, cc.total_chapters_count, tc.translated_chapters").
		Joins("LEFT JOIN (?) as mc ON mc.novel_id = novels.id", maxChapterIDSubquery).
		Joins("LEFT JOIN chapters as c ON mc.id = c.id").
		Joins("LEFT JOIN (?) as cc ON cc.novel_id = novels.id", chapterCountSubquery).
		Joins("LEFT JOIN (?) as tc ON tc.novel_id = novels.id", translatedChapterCountSubquery).
		Where("novels.deleted_at IS NULL").
		Order("novels.created_at DESC")

	if err := query.Count(&totalNovels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error counting novels"})
		return
	}

	if err := query.Limit(pageSize).Offset(offset).Scan(&novelResponses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching novels"})
		return
	}

	for i := range novelResponses {
		if novelResponses[i].TranslatedChapters == novelResponses[i].TotalChaptersCount {
			novelResponses[i].TranslationStatus = "completed"
		} else {
			novelResponses[i].TranslationStatus = "in_progress"
		}
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

// ListMissingTranslations godoc
// @Summary List chapters with missing translations
// @Description Get a list of chapters that have missing or incomplete translations
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {array} ChapterResponse
// @Failure 500 {object} string "Internal server error"
// @Router /admin/chapters/missing-translation [get]
func (h *NovelHandler) ListMissingTranslations(c *gin.Context) {
	var chapters []models.Chapter
	result := h.DB.Where("translated_content IS NULL OR translation_status <> 'completed'").Find(&chapters)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching chapters"})
		return
	}
	c.JSON(http.StatusOK, chapters)
}

// ReTranslateChapters godoc
// @Summary Re-translate chapters
// @Description Initiate re-translation of chapters with missing or incomplete translations
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} string "Internal server error"
// @Router /admin/chapters/missing-translation [post]
func (h *NovelHandler) ReTranslateChapters(c *gin.Context) {
	var chapters []models.Chapter
	result := h.DB.Where("translated_content IS NULL OR translation_status <> 'completed'").Find(&chapters)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching chapters"})
		return
	}
	successCount := 0
	failureCount := 0
	failures := make([]uint, 0)

	for _, chapter := range chapters {
		err := h.TranslateChapter(&chapter)
		if err != nil {
			failureCount++
			failures = append(failures, chapter.ID)
			continue
		}
		successCount++
	}

	response := gin.H{
		"message":      "Re-translation process initiated",
		"successCount": successCount,
		"failureCount": failureCount,
		"failures":     failures,
	}

	c.JSON(http.StatusOK, response)
}

func (h *NovelHandler) TranslateChapter(chapter *models.Chapter) error {
	content := *chapter.Content
	translatedContent, err := lib.Translate(content)
	if err != nil {
		return err
	}

	chapter.TranslatedContent = translatedContent
	chapter.TranslationStatus = "completed"

	result := h.DB.Save(chapter)
	return result.Error
}

// MigrateNovelThumbnails godoc
// @Summary Migrate novel thumbnails
// @Description Migrate novel thumbnails to a new storage system
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} string "Internal server error"
// @Router /admin/migrate-thumbnail [post]

func (h *NovelHandler) MigrateNovelThumbnails(c *gin.Context) {
	var novels []models.Novel
	result := h.DB.Find(&novels, "thumbnail IS NOT NULL")
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching novels"})
		return
	}

	for _, novel := range novels {
		if novel.Thumbnail != nil {
			s3URL, err := utils.DownloadAndUploadImage(*novel.Thumbnail, "cover")
			if err != nil {
				log.Printf("Error migrating thumbnail for novel ID %d: %v", novel.ID, err)
				continue
			}
			novel.Thumbnail = &s3URL
			result := h.DB.Save(&novel)
			if result.Error != nil {
				log.Printf("Error updating novel ID %d: %v", novel.ID, result.Error)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Migration completed"})
}

// RetranslateChapters godoc
// @Summary Retranslate chapters
// @Description Initiate retranslation of all chapters
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} string "Retranslation process initiated"
// @Failure 500 {object} string "Internal server error"
// @Router /admin/retranslate [post]
func (h *NovelHandler) RetranslateChapters(c *gin.Context) {
	if err := h.Worker.RetranslateChapters(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (h *NovelHandler) ReSlugify(c *gin.Context) {
	var novels []models.Novel
	result := h.DB.Find(&novels)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching novels"})
		return
	}

	for _, novel := range novels {
		slug := utils.Slugify(*novel.Title)
		novel.Slug = &slug
		result := h.DB.Save(&novel)
		if result.Error != nil {
			log.Printf("Error updating novel ID %d: %v", novel.ID, result.Error)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Migration completed"})
}

func (h *NovelHandler) ReSlugChapter(c *gin.Context) {
	const batchSize = 1000
	var offset int

	for {
		var chapters []models.Chapter
		result := h.DB.Limit(batchSize).Offset(offset).Find(&chapters)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching chapters"})
			return
		}

		if len(chapters) == 0 {
			break
		}

		for _, chapter := range chapters {
			slug := utils.Slugify(*chapter.TranslatedTitle)
			chapter.Slug = slug
			result := h.DB.Save(&chapter)
			if result.Error != nil {
				log.Printf("Error updating chapter ID %d: %v", chapter.ID, result.Error)
			}
		}

		offset += batchSize
	}

	c.JSON(http.StatusOK, gin.H{"message": "Migration completed"})
}
