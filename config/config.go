package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	ServerPort  string
	S3AccessKey string
	S3SecretKey string
	S3Endpoint  string
}

func LoadConfig() (*Config, error) {
	config := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		ServerPort:  os.Getenv("PORT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
	}

	// Check for required environment variables
	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}
	if config.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is not set")
	}
	if config.ServerPort == "" {
		return nil, fmt.Errorf("PORT environment variable is not set")
	}

	// Optional variables (comment out if not needed)
	// if config.S3AccessKey == "" {
	//     return nil, fmt.Errorf("S3_ACCESS_KEY environment variable is not set")
	// }
	// if config.S3SecretKey == "" {
	//     return nil, fmt.Errorf("S3_SECRET_KEY environment variable is not set")
	// }
	// if config.S3Endpoint == "" {
	//     return nil, fmt.Errorf("S3_ENDPOINT environment variable is not set")
	// }

	return config, nil
}
