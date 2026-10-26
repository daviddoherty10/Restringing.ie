package config

import (
	"errors"
	"net/url"
	"os"
)

type Config struct {
	APIAddr        string
	DatabaseURL    string
	FrontendOrigin string
}

func Load() (Config, error) {
	cfg := Config{
		APIAddr:        os.Getenv("API_ADDR"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
	}

	if cfg.APIAddr == "" {
		cfg.APIAddr = ":8081"
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.FrontendOrigin == "" {
		cfg.FrontendOrigin = "http://localhost:5173"
	}
	origin, err := url.Parse(cfg.FrontendOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return Config{}, errors.New("FRONTEND_ORIGIN must be an exact HTTP(S) origin without a path")
	}

	return cfg, nil
}
