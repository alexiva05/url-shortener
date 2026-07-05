package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/alexiva05/url-shortener/internal/config"
	"github.com/alexiva05/url-shortener/internal/http-server/handlers/url/save"
	mwLogger "github.com/alexiva05/url-shortener/internal/http-server/middleware/logger"
	mwURLFormat "github.com/alexiva05/url-shortener/internal/http-server/middleware/url"
	"github.com/alexiva05/url-shortener/internal/storage/postgres"
	"github.com/alexiva05/url-shortener/lib/logger/sl"
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
		log.Error("failed to create connection pool", sl.Err(err))
		os.Exit(1)
	}
	defer db.Close()

	log.Info("Connected to database")

	storage, err := postgres.NewStorage(ctx, db)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(mwURLFormat.URLFormat)

	router.Post("/url", save.New(log, storage))

	log.Info("starting server", slog.String("address", cfg.Address))

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err := srv.ListenAndServe(); err != nil {
		if errors.As(err, &http.ErrServerClosed) {
			log.Info("shortener server closed")
		}
		log.Error("failed to start server", sl.Err(err))
	}

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
