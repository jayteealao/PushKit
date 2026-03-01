package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/pushkit/backend/internal/api"
	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/config"
	"github.com/pushkit/backend/internal/db"
	s3client "github.com/pushkit/backend/internal/s3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.CreateTables(database); err != nil {
		slog.Error("create tables", "err", err)
		os.Exit(1)
	}

	s3, err := s3client.NewClient(cfg)
	if err != nil {
		slog.Error("create S3 client", "err", err)
		os.Exit(1)
	}

	uploadHandler := &api.UploadHandler{DB: database, S3: s3}
	fileHandler := &api.FileHandler{DB: database, S3: s3}

	r := chi.NewRouter()

	// Global middleware.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger)

	// Health check (unauthenticated).
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg))
		r.Mount("/v1/uploads", uploadHandler.Routes())
		r.Mount("/v1/files", fileHandler.Routes())
	})

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		)
		next.ServeHTTP(w, r)
	})
}
