package config

import "os"

type Config struct {
	DatabaseURL string
	RedisURL    string
	ServerPort  string
}

func LoadConfig() *Config {
	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		ServerPort:  os.Getenv("PORT"),
	}
}
