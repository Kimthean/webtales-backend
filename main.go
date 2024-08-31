package main

import (
	"context"
	"go-novel/config"
	"go-novel/crawler"
	"go-novel/db"
	auth "go-novel/handler/auth"
	genre "go-novel/handler/genre"
	novel "go-novel/handler/novel"
	"go-novel/handler/user"
	"go-novel/middleware"
	"go-novel/models"
	"go-novel/utils"
	"go-novel/worker"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/go-redis/redis/v8"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&models.Novel{}, &models.Chapter{}, &models.User{}, &models.Genre{}, &models.Tag{})

	redisURL := cfg.RedisURL
	redisURL = strings.TrimPrefix(redisURL, "redis://")
	parts := strings.Split(redisURL, "@")
	if len(parts) != 2 {
		log.Fatalf("Invalid Redis URL format: %s", cfg.RedisURL)
	}
	password := strings.TrimPrefix(parts[0], ":")
	address := parts[1]
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	// rdb := redis.NewClient(&redis.Options{
	// 	Addr: cfg.RedisURL,
	// })

	err = utils.InitS3()
	if err != nil {
		log.Fatalf("Failed to initialize S3: %v", err)
	}

	// Crawler and worker setup
	crawler := crawler.NewCrawler()
	w := worker.NewWorker(crawler, db, rdb)
	go w.Start(context.Background())

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://webtalesmtl.xyz", "http://localhost:4321"}, // Allowed origins
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},                     // Allowed methods
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},          // Allowed headers
		ExposeHeaders:    []string{"Content-Length"},                                   // Headers that can be exposed to the browser
		AllowCredentials: true,                                                         // Allow credentials (cookies, authorization headers, etc.)
		AllowOriginFunc: func(origin string) bool {
			// Custom logic to allow specific origins
			return origin == "https://specific-allowed-origin.com"
		},
		MaxAge: 12 * time.Hour,
	}))

	novelHandler := &novel.NovelHandler{DB: db, Worker: w}
	genreHandler := &genre.GenreHandler{DB: db}
	authHandler := &auth.AuthHandler{DB: db}

	// Novel routes
	novelRoutes := r.Group("/novel")
	{
		novelRoutes.GET("/:id", novelHandler.GetNovel)
		novelRoutes.GET("/:id/chapters", novelHandler.GetNovelChapters)
		novelRoutes.GET("/all", novelHandler.GetNovels)
		novelRoutes.GET("/latest", novelHandler.GetLatestNovels)
		novelRoutes.GET("/latest-update", novelHandler.GetLatestUpdate)
		novelRoutes.GET("", novelHandler.GetPaginatedNovels)
		novelRoutes.GET("/:id/chapter/:number", novelHandler.GetChapterByID)
		novelRoutes.GET("/:id/paginate-chapters", novelHandler.GetNovelChaptersWithPage)
		novelRoutes.GET("/chapters-stats/:id", novelHandler.GetNovelTranslationStatus)
		novelRoutes.GET("/search", novelHandler.SearchNovels)
		novelRoutes.POST("/convert-epub/:id", func(c *gin.Context) {
			novelID := c.Param("id")
			err := w.EnqueueNovelForConversion(novelID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Conversion process initiated"})
		})
	}

	// Genre routes
	genreRoutes := r.Group("/genre")
	{
		genreRoutes.GET("", genreHandler.GetGenres)
		genreRoutes.GET("/:novelID", genreHandler.GetNovelGenres)
		genreRoutes.POST("", genreHandler.CreateGenre)
		genreRoutes.POST("/:novelID/genre/:genreID", genreHandler.AddGenreToNovel)
	}

	adminRoutes := r.Group("/admin")
	adminRoutes.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
	{
		adminRoutes.GET("/chapters/missing-translation", novelHandler.ListMissingTranslations)
		adminRoutes.POST("/chapters/missing-translation", novelHandler.ReTranslateChapters)
		adminRoutes.POST("/migrate-thumbnail", novelHandler.MigrateNovelThumbnails)
		adminRoutes.DELETE("/:id", novelHandler.DeleteNovelByID)
		adminRoutes.GET("/users", authHandler.GetAllUsers)
		adminRoutes.DELETE("/novel/:id", novelHandler.DeleteNovelByID)
		adminRoutes.POST("/retranslate", novelHandler.RetranslateChapters)
		adminRoutes.DELETE("/genre/:id", genreHandler.DeleteGenre)

		adminRoutes.POST("/update/:id", func(c *gin.Context) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
				return
			}
			err = w.EnqueueUpdate(uint(id))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Update process initiated"})
		})
		adminRoutes.POST("/crawl", func(c *gin.Context) {
			url := c.Query("url")
			err := w.EnqueueNovel(url)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to enqueue novel")
				return
			}
			c.String(http.StatusOK, "Novel queued for crawling")
		})

	}

	// Auth routes
	authRoutes := r.Group("auth")
	{
		authRoutes.POST("/signup", authHandler.SignUp)
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/google/callback", authHandler.GoogleCallback)
	}

	userHandler := &user.UserHandler{DB: db}

	userRoutes := r.Group("/user")
	userRoutes.Use(middleware.AuthMiddleware())
	{
		userRoutes.GET("/me", userHandler.GetCurrentUser)
		userRoutes.PUT("/profile", userHandler.UpdateProfile)
		userRoutes.PUT("/change-password", userHandler.ChangePassword)
		userRoutes.POST("/profile-picture", userHandler.UploadProfilePicture)
		userRoutes.POST("/bookmark/:novelID", userHandler.AddNovelToBookmark)
		userRoutes.GET("/bookmarks", userHandler.GetUserBookmarks)
	}

	// Health check
	r.GET("/", func(c *gin.Context) {
		var err error
		var version string
		err = db.Raw("SELECT VERSION()").Scan(&version).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database is unreachable"})
			return
		}

		if _, err = rdb.Ping(context.Background()).Result(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis is unreachable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "All services are healthy!",
		})
	})

	r.Run(":" + cfg.ServerPort)
}
