package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/pushkit/backend/internal/api"
	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/config"
	"github.com/pushkit/backend/internal/db"
	"github.com/pushkit/backend/internal/events"
	s3client "github.com/pushkit/backend/internal/s3"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
var Version = "dev"

// Version output format: "pushkit-server <version>\n"
// This intentionally differs from the CLI (Cobra default: "pushkit version <version>")
// because the server is a dependency-light service binary that uses stdlib flag rather
// than Cobra. The divergence is documented; do not unify without weighing the added
// dependency cost on the server binary.
func printVersion(w io.Writer, v string) {
	fmt.Fprintf(w, "pushkit-server %s\n", v)
}

// hasVersionFlag reports whether -v or --version appears in args without
// invoking flag.Parse, so the version check is always safe to run first,
// before any DB/S3 initialisation.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "-v" || a == "--v" || a == "-version" || a == "--version" {
			return true
		}
	}
	return false
}

// hasDBCheckFlag reports whether --db-check appears in args.
// When present, the server opens its SQLite database and exits without
// starting the HTTP listener or loading the S3/API-key configuration.
// This is used as a CI smoke-test hook to verify that the binary was
// compiled with cgo enabled (a CGO_ENABLED=0 stub panics on DB open).
func hasDBCheckFlag(args []string) bool {
	for _, a := range args {
		if a == "--db-check" {
			return true
		}
	}
	return false
}

func main() {
	// Version check runs before everything else (no DB/S3 side-effects).
	if hasVersionFlag(os.Args[1:]) {
		printVersion(os.Stdout, Version)
		os.Exit(0)
	}

	// DB-check flag: open SQLite and exit. Used by CI to verify cgo is enabled.
	if hasDBCheckFlag(os.Args[1:]) {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "pushkit-smoke.db"
		}
		database, err := db.Open(dsn)
		if err != nil {
			slog.Error("db-check: open database", "err", err)
			os.Exit(1)
		}
		database.Close()
		fmt.Fprintln(os.Stdout, "db-check: ok")
		os.Exit(0)
	}

	// M-08: Use a private FlagSet with ContinueOnError so that unknown flags
	// passed by a service manager or wrapper do NOT crash the server. The server
	// reads all meaningful configuration from the environment; CLI args are only
	// used for --version above. Any parse error here is logged and ignored.
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	if err := fs.Parse(os.Args[1:]); err != nil {
		// Unknown flags are tolerated — log at debug level and continue.
		slog.Debug("ignoring unrecognised flags", "err", err)
	}

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

	// In-process event hub for the SSE stream; holds no background goroutine.
	hub := events.NewHub()

	uploadHandler := &api.UploadHandler{DB: database, S3: s3, Events: hub}
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
		r.Handle("/v1/events", &events.Handler{Hub: hub})
	})

	// M-11: Optional TLS. When TLS_CERT_FILE and TLS_KEY_FILE are both set,
	// start HTTPS; otherwise fall back to plain HTTP. OFF by default.
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		slog.Info("starting server (TLS)", "addr", cfg.ListenAddr)
		if err := http.ListenAndServeTLS(cfg.ListenAddr, cfg.TLSCertFile, cfg.TLSKeyFile, r); err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	} else {
		slog.Info("starting server (plaintext)", "addr", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
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
