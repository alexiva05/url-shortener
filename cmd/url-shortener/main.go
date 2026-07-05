package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/alexiva05/url-shortener/internal/config"
	"github.com/alexiva05/url-shortener/internal/storage/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := setupLogger(cfg.Env)
	log.Info("Starting URL Shortener", slog.String("env", cfg.Env))
	log.Debug("debug log enabled")

	db, err := pgxpool.New(ctx, cfg.Storage)
	if err != nil {
		log.Error("failed to create connection pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	log.Info("Connected to database")

	storage, err := postgres.NewStorage(ctx, db)
	if err != nil {
		log.Error("failed to init storage", slog.String("error", err.Error()))
		os.Exit(1)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)

	_ = storage

	fmt.Println(cfg)
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
