package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/config"
	"github.com/daviddoherty10/Restringing.ie/backend/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Println("Connected to PostgreSQL")

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", cfg.APIAddr)

	return fmt.Errorf("HTTP server stopped: %w", server.ListenAndServe())
}
