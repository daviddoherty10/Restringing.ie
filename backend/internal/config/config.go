package config

import (
	"errors"
	"os"
)

type Config struct {
	APIAddr     string
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		APIAddr:     os.Getenv("API_ADDR"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.APIAddr == "" {
		cfg.APIAddr = ":8081"
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}
