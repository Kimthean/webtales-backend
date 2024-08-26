package services

import (
	"errors"
	"go-novel/models"

	"gorm.io/gorm"
)

type GenreService struct {
	DB *gorm.DB
}

func NewGenreService(db *gorm.DB) *GenreService {
	return &GenreService{DB: db}
}

func (s *GenreService) AddGenreToNovel(novelID uint, genreID uint) error {
	var novel models.Novel
	if err := s.DB.First(&novel, novelID).Error; err != nil {
		return err
	}

	var genre models.Genre
	if err := s.DB.First(&genre, genreID).Error; err != nil {
		return err
	}

	return s.DB.Model(&novel).Association("Genres").Append(&genre)
}

func (s *GenreService) DeleteGenreFromNovel(novelID uint, genreID uint) error {
	var novel models.Novel
	if err := s.DB.First(&novel, novelID).Error; err != nil {
		return err
	}

	var genre models.Genre
	if err := s.DB.First(&genre, genreID).Error; err != nil {
		return err
	}

	return s.DB.Model(&novel).Association("Genres").Delete(&genre)
}

func (s *GenreService) GetNovelGenres(novelID uint) ([]*models.Genre, error) {
	var novel models.Novel
	if err := s.DB.Preload("Genres").First(&novel, novelID).Error; err != nil {
		return nil, err
	}
	return novel.Genres, nil
}

func (s *GenreService) DeleteGenre(genreID uint) error {
	result := s.DB.Delete(&models.Genre{}, genreID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("genre not found")
	}
	return nil
}

func (s *GenreService) GetAllGenres() ([]*models.Genre, error) {
	var genres []*models.Genre
	if err := s.DB.Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}
