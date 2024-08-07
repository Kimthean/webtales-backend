package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	ServerPort  string
	S3AccessKey string
	S3SecretKey string
	S3Endpoint  string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		ServerPort:  os.Getenv("PORT"),
		// S3AccessKey: os.Getenv("S3_SECRET_KEY"),
		// S3SecretKey: os.Getenv("S3_ACCESS_KEY"),
		// S3Endpoint:  os.Getenv("S3_ENDPOINT"),
	}
}
